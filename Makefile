# Variables
FRONTEND_DIR = mod.abuse_glines/frontent-glines
BUILD_DIR = $(FRONTEND_DIR)/dist
NPM = npm

.PHONY: all clean build deploy-frontend

# Default target
all: build

# Build the frontend
build:
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
