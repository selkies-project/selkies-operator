# Pod broker web interface

## Local development

Requirements:
- kubectl
- nodejs

1. Install node modules:

```bash
npm install
```

2. Start dev server with port-forward to `pod-broker-0` in current Kubernetes namespace:

```bash
./dev_server.sh
```

3. Open URL displayed in dev server output.