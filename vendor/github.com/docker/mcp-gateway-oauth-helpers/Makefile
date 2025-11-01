# Makefile for mcp-gateway-oauth-helpers library

.PHONY: format lint test clean

# Format Go code using Docker
format:
	docker buildx build --target=format -o . .

# Run linting using Docker
lint:
	docker buildx build --target=lint --platform=linux,darwin,windows .

# Run linting for specific platform
lint-%:
	docker buildx build --target=lint --platform=$* .

# Run tests using Docker
test:
	docker buildx build --target=test .

# Clean build cache
clean:
	docker buildx prune -f