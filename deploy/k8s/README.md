# StreamForge Local Kubernetes Deployment

This manifest set targets Windows local Kubernetes, such as Docker Desktop Kubernetes.
It deploys StreamForge frontend, Java user-service, Go media-service, Redis, and LiveKit.
MySQL is expected to be a cloud or externally managed database.

## 1. Prepare Cloud MySQL

Create the `streamforge` database and import:

```text
deploy/mysql/init/01-users.sql
```

Allow your Windows machine's outbound IP to access the cloud MySQL `3306` port.

Before applying manifests, replace these placeholders:

```text
deploy/k8s/configmap.yaml
  MYSQL_HOST: "replace-with-cloud-mysql-host"

deploy/k8s/secret.yaml
  MYSQL_PASSWORD: "replace-with-cloud-mysql-password"
```

Adjust `MYSQL_USERNAME` if your cloud database user is not `streamforge`.

## 2. Build Local Images

Run from the repository root:

```powershell
docker build -t streamforge/frontend:local `
  --build-arg VITE_USER_API_BASE_URL=http://localhost:31081 `
  --build-arg VITE_MEDIA_API_BASE_URL=http://localhost:31080 `
  .\frontend

docker build -t streamforge/media-service:local .\media-service
docker build -t streamforge/user-service:local .\user-service
```

The manifests use `imagePullPolicy: IfNotPresent`, so Docker Desktop Kubernetes can use these local images.

## 3. Apply Manifests

```powershell
kubectl apply -f .\deploy\k8s\
kubectl get pods -n streamforge -w
kubectl get svc -n streamforge
```

## 4. Local Ports

| Service | URL |
|:---|:---|
| frontend | `http://localhost:31000` |
| user-service | `http://localhost:31081` |
| media-service | `http://localhost:31080` |
| LiveKit signal | `ws://localhost:30880` |
| LiveKit TCP fallback | `localhost:30881` |
| LiveKit RTC UDP | `localhost:30000-30010/udp` |

## 5. Validate

```powershell
curl http://localhost:31081/health
curl http://localhost:31080/health
```

Then open:

```text
http://localhost:31000
```

Register/login, create a room, and join the same room from two browser tabs.
For local HTTP camera and microphone access, use `localhost`.

