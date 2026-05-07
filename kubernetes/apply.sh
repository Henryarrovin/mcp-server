#!/bin/bash
set -e

echo "Creating namespace..."
kubectl apply -f namespace.yaml

echo "Creating secrets..."
if [ ! -f /workspace/.env.secrets ]; then
    echo "▶ .env.secrets not found, generating..."

    CANONICAL_SECRET=$(openssl rand -hex 32)

    cat > /workspace/.env.secrets << EOF
AUTH_DB_PASSWORD=postgres
PAYMENT_DB_PASSWORD=postgres
AUTH_JWT_ACCESS_SECRET=$(openssl rand -hex 32)
AUTH_JWT_REFRESH_SECRET=$(openssl rand -hex 32)
AUTH_JWT_CANONICAL_SECRET=${CANONICAL_SECRET}
PAYMENT_AUTH_GRPC_CANONICAL_SECRET=${CANONICAL_SECRET}
EOF
    echo "Secrets generated"
fi

echo "Secrets file contents:"
cat /workspace/.env.secrets

kubectl create secret generic auth-secrets \
  --namespace auth \
  --from-env-file=/workspace/.env.secrets \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic payment-secrets \
  --namespace auth \
  --from-env-file=/workspace/.env.secrets \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Secret created"
kubectl get secret auth-secrets -n auth
kubectl get secret payment-secrets -n auth

echo "Creating configmap..."
kubectl apply -f configmap.yaml

echo "Deploying postgres..."
kubectl apply -f postgres/pvc.yaml
kubectl apply -f postgres/deployment.yaml
kubectl apply -f postgres/service.yaml

echo "Deploying redis..."
kubectl apply -f redis/pvc.yaml
kubectl apply -f redis/deployment.yaml
kubectl apply -f redis/service.yaml

echo "Deploying zookeeper..."
kubectl apply -f zookeeper/deployment.yaml
kubectl apply -f zookeeper/service.yaml

echo "Deploying kafka..."
kubectl apply -f kafka/deployment.yaml
kubectl apply -f kafka/service.yaml

echo "Creating logs PVC..."
kubectl apply -f logs/pvc.yaml

echo "Waiting for postgres..."
kubectl wait --namespace auth \
  --for=condition=ready pod \
  --selector=app=postgres \
  --timeout=90s

echo "Waiting for kafka..."
kubectl wait --namespace auth \
  --for=condition=ready pod \
  --selector=app=kafka \
  --timeout=90s

echo "Deploying Ollama..."
kubectl apply -f ollama/pvc.yaml
kubectl apply -f ollama/deployment.yaml
kubectl apply -f ollama/service.yaml

echo "Waiting for Ollama to be ready..."
kubectl wait --namespace auth \
  --for=condition=ready pod \
  --selector=app=ollama \
  --timeout=120s
 
echo "Pulling llama3.2 model (this takes a few minutes on first run)..."
kubectl apply -f ollama/pull-model-job.yaml

echo "Creating payment_db database..."
kubectl exec -n auth deployment/postgres -- \
  psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname='payment_db'" | \
  grep -q 1 || \
  kubectl exec -n auth deployment/postgres -- \
  psql -U postgres -c "CREATE DATABASE payment_db;"

echo "Creating auth_db database..."
kubectl exec -n auth deployment/postgres -- \
  psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname='auth_db'" | \
  grep -q 1 || \
  kubectl exec -n auth deployment/postgres -- \
  psql -U postgres -c "CREATE DATABASE auth_db;"

echo "Deploying auth-service..."
kubectl apply -f auth-service/deployment.yaml
kubectl apply -f auth-service/service.yaml

echo "Deploying payment-gateway..."
kubectl apply -f payment-service/deployment.yaml
kubectl apply -f payment-service/service.yaml

echo "Removing ingress admission webhook..."
kubectl delete validatingwebhookconfiguration ingress-nginx-admission --ignore-not-found=true

echo "Applying ingresses..."
kubectl apply -f auth-service/ingress.yaml
kubectl apply -f payment-service/ingress.yaml
kubectl apply -f mcp-server/ingress.yaml

echo "Deploying mock razorpay server..."
kubectl apply -f mock-razorpay/deployment.yaml
kubectl apply -f mock-razorpay/service.yaml

echo "Deploying MCP Server..."
kubectl apply -f mcp-server/deployment.yaml
kubectl apply -f mcp-server/service.yaml
kubectl apply -f mcp-server/service-account.yaml
kubectl apply -f mcp-server/cluster-role.yaml
 
echo "Waiting for MCP server to be ready..."
kubectl wait --namespace auth \
  --for=condition=ready pod \
  --selector=app=mcp-server \
  --timeout=60s

echo "Done!"
kubectl get all -n auth

echo "▶ Starting port forward for ingress-nginx-controller..."
kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 80:80 --address 0.0.0.0 &

echo "Building MCP CLI..."

kubectl exec -n auth deployment/mcp-server -- sh -c "
cd /app &&
make cli-build
"

echo "Making 'mcp' command globally runnable..."

kubectl exec -n auth deployment/mcp-server -- sh -c "
chmod +x /app/bin/mcp-cli &&
ln -sf /app/bin/mcp-cli /usr/local/bin/mcp
"

echo "Testing MCP CLI..."
kubectl exec -n auth deployment/mcp-server -- mcp --help || true

echo "Creating MCP CLI env file..."

kubectl exec -n auth deployment/mcp-server -- sh -c '
cat > /root/.mcp.env << EOF
OLLAMA_URL=http://ollama-service:11434
OLLAMA_MODEL=qwen2.5:1.5b
MCP_SSE_URL=http://mcp-server-service:8085/sse
MCP_CHAT_URL=http://mcp-server-service:8085/chat
EOF
'