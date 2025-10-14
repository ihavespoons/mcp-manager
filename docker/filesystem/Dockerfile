# Dockerfile for MCP Filesystem Server
FROM node:24-alpine

# Install the MCP filesystem server
RUN npm install -g @modelcontextprotocol/server-filesystem

# Set the entrypoint to the MCP server
# The directory will be passed as an argument
ENTRYPOINT ["npx", "-y", "@modelcontextprotocol/server-filesystem"]

# Default to /workspace if no directory specified
CMD ["/workspace"]
