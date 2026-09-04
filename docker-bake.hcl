variable "TAG" {
  default = "latest"
}
variable "COMMIT" {
  default = "head"
}
variable "ARCH" {
  default = split("/", BAKE_LOCAL_PLATFORM)[1]
}
variable "OS" {
  default = "linux"
}
variable "REPO" {
  default = "rancher"
}
variable "DEST_DIR" {
  default = ""
}

group "default" {
  targets = ["server", "agent", "demo"]
}

target "_base" {
  context = "."
  dockerfile = "./package/Dockerfile"
  platforms = ["${OS}/${ARCH}"]
  args = {
    VERSION = "${TAG}"
    COMMIT = "${COMMIT}"
    ARCH = "${ARCH}"
    RANCHER_REPO = "${REPO}"
  }
}

# This empty target is merged with the bake-file-labels JSON output from
# docker/metadata-action, allowing downstream targets to inherit OCI labels dynamically.
target "docker-metadata-action" {}

target "server" {
  inherits = ["_base", "docker-metadata-action"]
  target = "server"
  tags = ["${REPO}/rancher:${TAG}-${ARCH}"]
  output = DEST_DIR == "" ? ["type=docker"] : ["type=docker,dest=${DEST_DIR}/rancher-${OS}-${ARCH}.tar"]
}

target "agent" {
  inherits = ["_base", "docker-metadata-action"]
  target = "agent"
  tags = ["${REPO}/rancher-agent:${TAG}-${ARCH}"]
  output = DEST_DIR == "" ? ["type=docker"] : ["type=docker,dest=${DEST_DIR}/rancher-agent-${OS}-${ARCH}.tar"]
}

target "demo" {
  inherits = ["_base", "docker-metadata-action"]
  target = "demo"
  tags = ["${REPO}/rancher-demo:${TAG}-${ARCH}"]
  output = DEST_DIR == "" ? ["type=docker"] : ["type=docker,dest=${DEST_DIR}/rancher-demo-${OS}-${ARCH}.tar"]
}
