### Task 4: Add Nginx Configuration And Operations Runbook

**Files:**
- Create: `deploy/nginx/api-payment.pikri.my.id.conf`
- Modify: `README.md:16-75,156-175`

**Interfaces:**
- Consumes: API listener `127.0.0.1:8080`, certificate paths under `/etc/letsencrypt/live/api-payment.pikri.my.id/`.
- Produces: Nginx HTTPS proxy for `api-payment.pikri.my.id` and repeatable first-deployment instructions.

- [ ] **Step 1: Create Nginx server configuration**

Create `deploy/nginx/api-payment.pikri.my.id.conf`:

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name api-payment.pikri.my.id;

    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name api-payment.pikri.my.id;

    ssl_certificate /etc/letsencrypt/live/api-payment.pikri.my.id/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api-payment.pikri.my.id/privkey.pem;

    client_max_body_size 1m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Request-ID $http_x_request_id;
        proxy_connect_timeout 5s;
        proxy_read_timeout 60s;
    }
}
```

- [ ] **Step 2: Document first-server setup and operations**

Append a `## VPS Deployment` section to `README.md` containing these commands and checks:

```bash
sudo install -d -m 0750 -o pik -g pik /home/pik/container/payment-sandbox
sudo docker network inspect postgres_default mongodb_mongodb_default redis_redis_default
ssh-keyscan -H <VPS_HOST> > /tmp/payment-sandbox-known-hosts
sudo cp deploy/nginx/api-payment.pikri.my.id.conf /etc/nginx/sites-available/api-payment.pikri.my.id
sudo ln -s /etc/nginx/sites-available/api-payment.pikri.my.id /etc/nginx/sites-enabled/api-payment.pikri.my.id
sudo nginx -t
sudo systemctl reload nginx
curl -fsS http://127.0.0.1:8080/api/v1/ping
curl -fsS https://api-payment.pikri.my.id/api/v1/ping
```

Document that `payment_sandbox_user` must own database `payment_sandbox` and schema `public`; otherwise the PostgreSQL `root` superuser must run:

```sql
ALTER DATABASE payment_sandbox OWNER TO payment_sandbox_user;
\c payment_sandbox
ALTER SCHEMA public OWNER TO payment_sandbox_user;
GRANT ALL ON SCHEMA public TO payment_sandbox_user;
```

Document GitHub secrets exactly: `VPS_HOST`, `VPS_SSH_PRIVATE_KEY`, `VPS_SSH_KNOWN_HOSTS`, `JWT_SECRET`, `DB_PASSWORD`, `MONGO_PASSWORD`, `GHCR_PULL_TOKEN`; document optional repository variable `VPS_SSH_PORT` defaulting to `22`.

Document manual rollback, replacing the SHA with an already-published tag:

```bash
printf 'IMAGE=ghcr.io/<repository-owner>/payment-sandbox/<previous-sha>\n' > /home/pik/container/payment-sandbox/.deploy.env
cd /home/pik/container/payment-sandbox
sudo docker compose -f docker-compose.yml --env-file .env --env-file .deploy.env up -d api
curl -fsS http://127.0.0.1:8080/api/v1/ping
```

- [ ] **Step 3: Validate host configuration on VPS**

Run on VPS after copying the Nginx file:

```bash
sudo nginx -t
sudo systemctl reload nginx
curl -fsS https://api-payment.pikri.my.id/api/v1/ping
```

Expected: Nginx test passes; public ping returns HTTP `200` with `{"data":{"status":"ok"}}`.

- [ ] **Step 4: Commit**

```bash
git add deploy/nginx/api-payment.pikri.my.id.conf README.md
git commit -m "docs(deploy): document VPS and Nginx setup"
```
