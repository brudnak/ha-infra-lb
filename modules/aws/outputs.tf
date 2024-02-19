# First HA Group
output "ha_1_server1_ip" {
  value = module.ha-1.server1_ip
}

output "ha_1_server2_ip" {
  value = module.ha-1.server2_ip
}

output "ha_1_server3_ip" {
  value = module.ha-1.server3_ip
}

output "ha_1_server1_private_ip" {
  value = module.ha-1.server1_private_ip
}

output "ha_1_server2_private_ip" {
  value = module.ha-1.server2_private_ip
}

output "ha_1_server3_private_ip" {
  value = module.ha-1.server3_private_ip
}

# Second HA Group
output "ha_2_server1_ip" {
  value = module.ha-2.server1_ip
}

output "ha_2_server2_ip" {
  value = module.ha-2.server2_ip
}

output "ha_2_server3_ip" {
  value = module.ha-2.server3_ip
}

output "ha_2_server1_private_ip" {
  value = module.ha-2.server1_private_ip
}

output "ha_2_server2_private_ip" {
  value = module.ha-2.server2_private_ip
}

output "ha_2_server3_private_ip" {
  value = module.ha-2.server3_private_ip
}

output "ha_1_aws_lb" {
  value = module.ha-1.aws_lb
}

output "ha_2_aws_lb" {
  value = module.ha-2.aws_lb
}
