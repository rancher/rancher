
## local dev testing instructions

Option 1: Full chart CI run with a live cluster

```bash
./scripts/charts/ci 
```

Option 2: Test runs against the chart only 

```bash
# install the helm plugin first - helm plugin install https://github.com/helm-unittest/helm-unittest.git
bash dev-scripts/helm-unittest.sh
```

