resource "random_pet" "random_pet" {

  keepers = {
    aws_prefix = "${var.aws_prefix}"
  }

  length    = 2
  separator = "-"
}

provider "aws" {
  region     = var.aws_region
  access_key = var.aws_access_key
  secret_key = var.aws_secret_key
}

resource "aws_instance" "aws_instance" {
  count                  = 3
  ami                    = var.aws_ami
  instance_type          = "t3a.medium"
  subnet_id              = var.aws_subnet_id
  vpc_security_group_ids = [var.aws_security_group_id]
  key_name               = var.aws_pem_key_name

  root_block_device {
    volume_size = 150
  }

  tags = {
    Name = "${random_pet.random_pet.keepers.aws_prefix}-${random_pet.random_pet.id}${formatdate("MMMDDYY", timestamp())}"
  }
}

resource "aws_lb_target_group" "aws_lb_target_group_80" {
  name        = "${var.aws_prefix}-80-${random_pet.random_pet.id}${formatdate("MMMDDYY", timestamp())}"
  port        = 80
  protocol    = "HTTP"
  target_type = "instance"
  vpc_id      = var.aws_vpc
  health_check {
    protocol          = "HTTP"
    port              = "traffic-port"
    healthy_threshold = 3
    interval          = 10
  }
}

resource "aws_lb_target_group" "aws_lb_target_group_443" {
  name        = "${var.aws_prefix}-443-${random_pet.random_pet.id}${formatdate("MMMDDYY", timestamp())}"
  port        = 443
  protocol    = "HTTPS"
  target_type = "instance"
  vpc_id      = var.aws_vpc
  health_check {
    protocol          = "HTTPS"
    port              = 443
    healthy_threshold = 3
    interval          = 10
  }
}

# attach instances to the target group 80
resource "aws_lb_target_group_attachment" "attach_tg_80" {
  count            = length(aws_instance.aws_instance)
  target_group_arn = aws_lb_target_group.aws_lb_target_group_80.arn
  target_id        = aws_instance.aws_instance[count.index].id
  port             = 80
}

# attach instances to the target group 443
resource "aws_lb_target_group_attachment" "attach_tg_443" {
  count            = length(aws_instance.aws_instance)
  target_group_arn = aws_lb_target_group.aws_lb_target_group_443.arn
  target_id        = aws_instance.aws_instance[count.index].id
  port             = 443
}

# create a load balancer
resource "aws_lb" "aws_lb" {
  load_balancer_type = "application"
  name               = "${var.aws_prefix}-nlb-${random_pet.random_pet.id}${formatdate("MMMDDYY", timestamp())}"
  internal           = false
  subnets            = [var.aws_subnet_a, var.aws_subnet_b, var.aws_subnet_c]
}

# add a listener for port 80
resource "aws_lb_listener" "aws_lb_listener_80" {
  load_balancer_arn = aws_lb.aws_lb.arn
  port              = "80"
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.aws_lb_target_group_80.arn
  }
}