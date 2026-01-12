 1. not respecting --context
 
 ./safekubectl delete pod -n istio-system --context main-correct-cluster-01 istiod-1-26-1-68f97bd684-zqlwd

⚠️   DANGEROUS OPERATION DETECTED
├── Operation: delete
├── Resource:  pod/istiod-1-26-1-68f97bd684-zqlwd
├── Namespace: istio-system
└── Cluster:   main-wrong-cluster-01

cannot use --context.
get pod with context is working as expected. output is similar with kubectl.

2. cordon doesn't need to show namespace

➜  ~ safekubectl cordon nodename

⚠️   DANGEROUS OPERATION DETECTED
├── Operation: cordon
├── Resource:  nodename
├── Namespace: default
└── Cluster:   clustername

Proceed? [y/N]: y
node/nodename already cordoned