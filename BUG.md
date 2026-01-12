 ./safekubectl delete pod -n istio-system --context main-correct-cluster-01 istiod-1-26-1-68f97bd684-zqlwd

⚠️   DANGEROUS OPERATION DETECTED
├── Operation: delete
├── Resource:  pod/istiod-1-26-1-68f97bd684-zqlwd
├── Namespace: istio-system
└── Cluster:   main-wrong-cluster-01

cannot use --context.
get pod with context is working as expected. output is similar with kubectl.