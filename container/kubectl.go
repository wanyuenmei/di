package container

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"golang.org/x/build/kubernetes"
	api "golang.org/x/build/kubernetes/api"
)

const KUBERNETES_BASE = "http://127.0.0.1:9000"
const DI_LABEL = "di-tag"

type kubectl struct {
	kubeClient *kubernetes.Client
	count      map[string]int32
}

func NewKubectl() controller {
	var err error
	var kubeClient *kubernetes.Client

	for {
		kubeClient, err = kubernetes.NewClient(KUBERNETES_BASE, &http.Client{})
		if err != nil {
			log.Warning("Failed to create kubernetes client: %s", err)
		} else {
			break
		}
		time.Sleep(10 * time.Second)
	}
	return kubectl{kubeClient: kubeClient, count: make(map[string]int32)}
}

func (k kubectl) getContainers() map[string][]Container {
	result := make(map[string][]Container)
	pods, err := k.kubeClient.GetPods(context.Background())
	if err != nil {
		log.Warning("Failed to get pods: %s", err)
		return result
	}

	for _, pod := range pods {
		c := Container{
			Name: pod.ObjectMeta.Name,
			IP:   pod.Status.PodIP,
		}
		l := append(result[pod.ObjectMeta.Labels[DI_LABEL]], c)
		result[pod.ObjectMeta.Labels[DI_LABEL]] = l
	}
	return result
}

func (k kubectl) bootContainers(name string, toBoot int) {
	if toBoot <= 0 {
		return
	}
	count := int(k.count[name])
	var wg sync.WaitGroup

	wg.Add(toBoot)
	defer wg.Wait()
	for i := count; i < count+toBoot; i++ {
		cName := fmt.Sprintf("di-%s-%d", name, i)
		ctx := context.Background()
		/* Since a pod is the atomic unit of kubernetes, we have to do this
		 * weird transform that maps containers to pods. E.g., if we say, "spawn
		 * 10 red containers", then this will be reflected as 10 separate pods
		 * in kubernetes. We do this primarily to allow more fine-grained
		 * control of things through the policy language. */
		go func() {
			defer wg.Done()
			_, err := k.kubeClient.RunPod(ctx, &api.Pod{
				TypeMeta: api.TypeMeta{APIVersion: "v1", Kind: "Pod"},
				ObjectMeta: api.ObjectMeta{
					Name: cName,
					Labels: map[string]string{
						DI_LABEL: name,
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{Name: cName,
							Image:   "ubuntu:14.04",
							Command: []string{"tail", "-f", "/dev/null"},
							TTY:     true},
					},
				},
			})
			if err != nil {
				log.Warning("Failed to run pod %s: %s", cName, err)
			} else {
				log.Info("Booted pod: %s", cName)
			}
		}()
	}
	k.count[name] += int32(toBoot)
}

func (k kubectl) terminateContainers(name string, toTerm []Container) {
	for _, c := range toTerm {
		ctx := context.Background()
		err := k.kubeClient.DeletePod(ctx, c.Name)
		if err != nil {
			log.Warning("Failed to delete pod %s: %s", c.Name, err)
		} else {
			log.Info("Deleted pod: %s", c.Name)
		}
	}
	k.count[name] -= int32(len(toTerm))
}
