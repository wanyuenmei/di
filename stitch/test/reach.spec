(label "a" (docker "ubuntu"))
(label "b" (docker "ubuntu"))
(label "c" (docker "ubuntu"))

(connect 22 "a" "b")
(connect 22 "b" "c")
