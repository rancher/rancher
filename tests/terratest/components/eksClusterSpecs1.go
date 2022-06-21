package components

var EKSClusterSpecs1 = `"
    subnets = var.aws_subnets
    security_groups = var.aws_security_groups
    private_access = var.private_access
    public_access = var.public_access`