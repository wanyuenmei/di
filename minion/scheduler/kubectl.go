package scheduler

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	"github.com/op/go-logging"
	"github.com/satori/go.uuid"
	"golang.org/x/build/kubernetes"
	api "golang.org/x/build/kubernetes/api"
	"golang.org/x/net/context"
)

var log = logging.MustGetLogger("scheduler")

const KUBERNETES_BASE = "http://127.0.0.1:9000"
const DI_LABEL = "di-tag"

type kubectl struct {
	kubeClient *kubernetes.Client
}

func NewKubectl() (scheduler, error) {
	body := `{"apiVersion":"v1","kind":"Namespace",` +
		`"metadata":{"name":"kube-system"}}`
	url := "http://127.0.0.1:9000/api/v1/namespaces"
	ctype := "application/json"
	_, err := http.Post(url, ctype, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewClient(KUBERNETES_BASE, &http.Client{})
	if err != nil {
		return nil, err
	}

	return kubectl{kubeClient: kubeClient}, nil
}

func (k kubectl) get() ([]Container, error) {
	pods, err := k.kubeClient.GetPods(ctx())
	if err != nil {
		return nil, err
	}

	var result []Container
	for _, pod := range pods {
		result = append(result, Container{
			ID:    pod.ObjectMeta.Name,
			IP:    pod.Status.PodIP,
			Label: pod.ObjectMeta.Labels[DI_LABEL],
		})
	}
	return result, err
}

func (k kubectl) boot(n int) {
	var wg sync.WaitGroup
	wg.Add(n)
	defer wg.Wait()

	for i := 0; i < n; i++ {
		go func() {
			k.bootContainer()
			wg.Done()
		}()
	}
}

func (k kubectl) terminate(ids []string) {
	for _, id := range ids {
		err := k.kubeClient.DeletePod(ctx(), id)
		if err != nil {
			log.Warning("Failed to delete pod %s: %s", id, err)
		} else {
			log.Info("Deleted pod: %s", id)
		}
	}
}

func (k kubectl) bootContainer() {
	id := uuid.NewV4().String()

	/* Since a pod is the atomic unit of kubernetes, we have to do this
	 * weird transform that maps containers to pods. E.g., if we say, "spawn
	 * 10 red containers", then this will be reflected as 10 separate pods
	 * in kubernetes. We do this primarily to allow more fine-grained
	 * control of things through the policy language. */
	_, err := k.kubeClient.RunPod(context.Background(), &api.Pod{
		TypeMeta: api.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: api.ObjectMeta{
			Name: id,
			Labels: map[string]string{
				DI_LABEL: id,
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{{
				Name:    id,
				Image:   "alpine",
				Command: []string{"tail", "-f", "/dev/null"},
			},
			},
		},
	})

	if err != nil {
		log.Warning("Failed to start pod %s: %s", id, err)
	} else {
		log.Info("Booted pod: %s", id)
	}
}

func ctx() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	return ctx
}
