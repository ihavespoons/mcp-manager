# Dockerfile for MCP Git Server
FROM node:24-alpine

# Install git
RUN apk add --no-cache git

# Install the MCP git server
RUN npm install -g @modelcontextprotocol/server-git

# Set the entrypoint to the MCP git server
ENTRYPOINT ["npx", "-y", "@modelcontextprotocol/server-git"]

# Default arguments
CMD ["--repository", "/workspace"]
