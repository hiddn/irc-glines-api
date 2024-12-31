# Variables
FRONTEND_DIR = mod.abuse_glines/frontent-glines
BUILD_DIR = $(FRONTEND_DIR)/dist
NPM = npm

.PHONY: all clean build-go build-frontend deploy-frontend

# Default target
all: build-go build-frontend deploy-frontend

# go
build-go:
	@echo "Building irc-glines-api..."
	go build .
	@echo "Building abuse_glines..."
	cd mod.abuse_glines && go build .
	@echo "Done."


# Build the frontend
build-frontend:
	@echo "Building the frontend..."
	cd $(FRONTEND_DIR) && $(NPM) install && $(NPM) run build
	@echo "Frontend build complete. Output is in $(BUILD_DIR)"

# Clean the build directory
clean:
	@echo "Cleaning the build directory..."
	rm -rf $(BUILD_DIR)
	@echo "Clean complete."

deploy-frontend:
	@echo "Deploying frontend to production..."
	./deploy-frontend.sh
	@echo "Done."
