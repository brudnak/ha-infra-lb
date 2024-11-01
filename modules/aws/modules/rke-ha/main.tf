# main.tf in modules/rke-ha directory

# Variables
variable "aws_prefix" {
  type        = string
  description = "Prefix for resource names"
}

variable "aws_vpc" {
  type        = string
  description = "VPC ID"
}

variable "aws_subnet_a" {
  type        = string
  description = "Subnet A ID"
}

variable "aws_subnet_b" {
  type        = string
  description = "Subnet B ID"
}

variable "aws_subnet_c" {
  type        = string
  description = "Subnet C ID"
}

variable "aws_ami" {
  type        = string
  description = "AMI ID for instances"
}

variable "aws_subnet_id" {
  type        = string
  description = "Subnet ID for instances"
}

variable "aws_security_group_id" {
  type        = string
  description = "Security group ID"
}

variable "aws_pem_key_name" {
  type        = string
  description = "Name of the PEM key for SSH access"
}

# Resources
resource "random_pet" "name" {
  keepers = {
    aws_prefix = var.aws_prefix
  }
  length    = 1
  separator = "-"
}

resource "random_id" "unique" {
  byte_length = 2
  keepers = {
    aws_prefix = var.aws_prefix
  }
}

locals {
  name_prefix = "${var.aws_prefix}-${random_pet.name.id}-${random_id.unique.hex}"
}

resource "aws_instance" "aws_instance" {
  count                  = 3
  ami                    = var.aws_ami
  instance_type          = "t3a.medium"
  subnet_id              = var.aws_subnet_id
  vpc_security_group_ids = [var.aws_security_group_id]
  key_name              = var.aws_pem_key_name

  root_block_device {
    volume_size = 150
  }

  tags = {
    Name = "${local.name_prefix}-${count.index + 1}"
  }
}

resource "aws_lb_target_group" "aws_lb_target_group_80" {
  name        = "${local.name_prefix}-80"
  port        = 80
  protocol    = "TCP"
  target_type = "instance"
  vpc_id      = var.aws_vpc
  health_check {
    protocol          = "TCP"
    port              = "traffic-port"
    healthy_threshold = 3
    interval          = 10
  }
}

resource "aws_lb_target_group" "aws_lb_target_group_443" {
  name        = "${local.name_prefix}-443"
  port        = 443
  protocol    = "TCP"
  target_type = "instance"
  vpc_id      = var.aws_vpc
  health_check {
    protocol          = "TCP"
    port              = 443
    healthy_threshold = 3
    interval          = 10
  }
}

resource "aws_lb_target_group_attachment" "attach_tg_80" {
  count            = length(aws_instance.aws_instance)
  target_group_arn = aws_lb_target_group.aws_lb_target_group_80.arn
  target_id        = aws_instance.aws_instance[count.index].id
  port             = 80
}

resource "aws_lb_target_group_attachment" "attach_tg_443" {
  count            = length(aws_instance.aws_instance)
  target_group_arn = aws_lb_target_group.aws_lb_target_group_443.arn
  target_id        = aws_instance.aws_instance[count.index].id
  port             = 443
}

resource "aws_lb" "aws_lb" {
  load_balancer_type = "network"
  name               = local.name_prefix
  internal           = false
  ip_address_type    = "ipv4"
  subnets            = [var.aws_subnet_a, var.aws_subnet_b, var.aws_subnet_c]
}

resource "aws_lb_listener" "aws_lb_listener_80" {
  load_balancer_arn = aws_lb.aws_lb.arn
  port              = "80"
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.aws_lb_target_group_80.arn
  }
}

resource "aws_lb_listener" "aws_lb_listener_443" {
  load_balancer_arn = aws_lb.aws_lb.arn
  port              = "443"
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.aws_lb_target_group_443.arn
  }
}

# Outputs
output "server1_ip" {
  value = aws_instance.aws_instance[0].public_ip
}

output "server2_ip" {
  value = aws_instance.aws_instance[1].public_ip
}

output "server3_ip" {
  value = aws_instance.aws_instance[2].public_ip
}

output "server1_private_ip" {
  value = aws_instance.aws_instance[0].private_ip
}

output "server2_private_ip" {
  value = aws_instance.aws_instance[1].private_ip
}

output "server3_private_ip" {
  value = aws_instance.aws_instance[2].private_ip
}

output "aws_lb" {
  value = aws_lb.aws_lb.dns_name
}
