# AODS Argo CD App Of Apps

이 디렉터리는 Argo CD에서 AODS를 root application 하나로 배포하기 위한 app-of-apps 진입점이다.

## 배포

```bash
kubectl apply -f deploy/argocd/aods-root.yaml
```

root application:

* `aods-root`: `deploy/argocd/apps` 아래 child `Application`들을 관리한다.

child applications:

* `aods-system`: `deploy/aods-system/overlays/argocd` 를 배포한다.

## 사전 준비 Secret

AODS backend는 런타임 secret을 Git에 저장하지 않는다. 배포 전 `aods-system` namespace에 아래 secret을 별도로 만들어 둔다.

```bash
kubectl create namespace aods-system --dry-run=client -o yaml | kubectl apply -f -

kubectl -n aods-system create secret generic aods-backend-secrets \
  --from-literal=AODS_GIT_REMOTE='https://<user>:<token>@github.com/Aolda/aods-manifest.git' \
  --from-literal=AODS_SECRET_STORE_MODE='iiv' \
  --from-literal=AODS_IIV_TOKEN='<iiv-token>' \
  --from-literal=AODS_MARIADB_DSN='<user>:<password>@tcp(<mariadb-service>.<namespace>.svc.cluster.local:3306)/aods?parseTime=true'
```

`AODS_MARIADB_DSN`은 선택 값이다. 없으면 deployment operation queue 없이 기존 동기 배포 경로로 동작한다.
prod overlay 는 backend `AODS_IIV_ADDR` 와 `aods-iiv` ClusterSecretStore server 를 `http://10.16.254.243:8200` 으로 고정한다. dev/testbed 에서는 같은 secret에 `AODS_IIV_ADDR` 를 별도로 넣어 환경별 IIV endpoint 를 바꿀 수 있다.

GHCR 이미지가 private 이면 image pull secret 도 별도로 만든다.

```bash
kubectl -n aods-system create secret docker-registry aods-registry-creds \
  --docker-server=ghcr.io \
  --docker-username='<github-user>' \
  --docker-password='<github-token>'
```

## MariaDB 포트 정책

기본 app-of-apps는 MariaDB를 새로 띄우지 않는다. 이미 `mariadb-dev` 같은 DB가 떠 있는 클러스터가 많기 때문에, AODS는 기존 MariaDB의 ClusterIP DNS를 DSN으로 참조한다.

권장 DSN 예시:

```text
wittadmin:<password>@tcp(mariadb-dev.mariadb.svc.cluster.local:3306)/aods?parseTime=true
```

별도 MariaDB를 추가로 띄워야 하면 NodePort를 쓰지 말고 `ClusterIP` 서비스로 노출한다. AODS backend는 클러스터 내부 DNS로만 DB에 접근하므로 host port나 NodePort를 점유할 필요가 없다.

## 포트 충돌 방지

`deploy/aods-system/overlays/argocd` 는 AODS backend/frontend 서비스를 명시적으로 `ClusterIP`로 고정한다.

* `aods-backend`: service port `8080`, ClusterIP only
* `aods-frontend`: service port `80`, ClusterIP only

외부 노출은 기존 Istio `VirtualService`와 ingress gateway가 담당한다. AODS app-of-apps 자체는 새 NodePort나 LoadBalancer 포트를 만들지 않는다.
