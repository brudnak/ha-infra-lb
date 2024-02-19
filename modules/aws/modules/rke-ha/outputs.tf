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
