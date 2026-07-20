dev:
	@echo "Starting development environment..."
	cd .docker && \
	ln -sf ../.env .env 2> /dev/null || true && \
	docker-compose -f docker-compose.dev.yml down && docker-compose -f docker-compose.dev.yml up --build
