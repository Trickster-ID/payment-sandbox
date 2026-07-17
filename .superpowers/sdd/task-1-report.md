Status: DONE
Commit SHA(s): 05f21a1
Test result: `go test ./app/middleware -run TestCORSMiddleware -v` and `docker build -t payment-sandbox:deployment-test .` passed.
Concerns: None.

Fix report:
- Status: DONE
- Scope: Added app-only `env_file: .env.docker` in `docker-compose.yml`; Dockerfile unchanged.
- Test command: `docker compose config --format json | jq -e '.services.app.environment.APP_ENV == "docker" and .services.app.environment.DB_HOST == "postgres" and .services.app.environment.REDIS_URL == "redis://redis:6379/0"'`
- Test output: `true`
- Concerns: `.env.docker` contains local development credentials. It is injected only at Compose runtime, not copied into the image.
