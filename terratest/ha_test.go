package test

import (
	"errors"
	"fmt"
	"github.com/brudnak/aws-ha-infra/terratest/hcl"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"log"
	"net"
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestHaSetup(t *testing.T) {

	viper.AddConfigPath("../")
	viper.SetConfigName("tool-config")
	viper.SetConfigType("yml")
	err := viper.ReadInConfig()

	if err != nil {
		log.Println("error reading config:", err)
	}

	hcl.GenAwsVar(
		viper.GetString("tf_vars.aws_access_key"),
		viper.GetString("tf_vars.aws_secret_key"),
		viper.GetString("tf_vars.aws_prefix"),
		viper.GetString("tf_vars.aws_vpc"),
		viper.GetString("tf_vars.aws_subnet_a"),
		viper.GetString("tf_vars.aws_subnet_b"),
		viper.GetString("tf_vars.aws_subnet_c"),
		viper.GetString("tf_vars.aws_ami"),
		viper.GetString("tf_vars.aws_subnet_id"),
		viper.GetString("tf_vars.aws_security_group_id"),
		viper.GetString("tf_vars.aws_pem_key_name"),
	)

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{

		TerraformDir: "../modules/aws",
		NoColor:      true,
	})

	terraform.InitAndApply(t, terraformOptions)

	// HA 1 section
	infra1Server1IPAddress := terraform.Output(t, terraformOptions, "ha_1_server1_ip")
	infra1Server2IPAddress := terraform.Output(t, terraformOptions, "ha_1_server2_ip")
	infra1Server3IPAddress := terraform.Output(t, terraformOptions, "ha_1_server3_ip")

	infra1Server1IPAddressPrivate := terraform.Output(t, terraformOptions, "ha_1_server1_private_ip")
	infra1Server2IPAddressPrivate := terraform.Output(t, terraformOptions, "ha_1_server2_private_ip")
	infra1Server3IPAddressPrivate := terraform.Output(t, terraformOptions, "ha_1_server3_private_ip")

	// HA 2 section
	infra2Server1IPAddress := terraform.Output(t, terraformOptions, "ha_2_server1_ip")
	infra2Server2IPAddress := terraform.Output(t, terraformOptions, "ha_2_server2_ip")
	infra2Server3IPAddress := terraform.Output(t, terraformOptions, "ha_2_server3_ip")

	infra2Server1IPAddressPrivate := terraform.Output(t, terraformOptions, "ha_2_server1_private_ip")
	infra2Server2IPAddressPrivate := terraform.Output(t, terraformOptions, "ha_2_server2_private_ip")
	infra2Server3IPAddressPrivate := terraform.Output(t, terraformOptions, "ha_2_server3_private_ip")

	// H1 Asserts
	assert.Equal(t, "valid", CheckIPAddress(infra1Server1IPAddress))
	assert.Equal(t, "valid", CheckIPAddress(infra1Server2IPAddress))
	assert.Equal(t, "valid", CheckIPAddress(infra1Server3IPAddress))
	assert.Equal(t, "valid", CheckIPAddress(infra1Server1IPAddressPrivate))
	assert.Equal(t, "valid", CheckIPAddress(infra1Server2IPAddressPrivate))
	assert.Equal(t, "valid", CheckIPAddress(infra1Server3IPAddressPrivate))

	// H2 Asserts
	assert.Equal(t, "valid", CheckIPAddress(infra2Server1IPAddress))
	assert.Equal(t, "valid", CheckIPAddress(infra2Server2IPAddress))
	assert.Equal(t, "valid", CheckIPAddress(infra2Server3IPAddress))
	assert.Equal(t, "valid", CheckIPAddress(infra2Server1IPAddressPrivate))
	assert.Equal(t, "valid", CheckIPAddress(infra2Server2IPAddressPrivate))
	assert.Equal(t, "valid", CheckIPAddress(infra2Server3IPAddressPrivate))

	infra1LB := terraform.Output(t, terraformOptions, "ha_1_aws_lb")
	infra2LB := terraform.Output(t, terraformOptions, "ha_2_aws_lb")

	pemPath := viper.GetString("local.pem_path")
	assert.NotEmpty(t, pemPath)

	CreateDir("high-availability-1")
	CreateDir("high-availability-2")

	WriteRkeConfig(
		pemPath,
		infra1Server1IPAddress,
		infra1Server2IPAddress,
		infra1Server3IPAddress,
		infra1Server1IPAddressPrivate,
		infra1Server2IPAddressPrivate,
		infra1Server3IPAddressPrivate,
		"high-availability-1/cluster.yml")

	WriteRkeConfig(
		pemPath,
		infra2Server1IPAddress,
		infra2Server2IPAddress,
		infra2Server3IPAddress,
		infra2Server1IPAddressPrivate,
		infra2Server2IPAddressPrivate,
		infra2Server3IPAddressPrivate,
		"high-availability-2/cluster.yml")

	bootstrapPassword := viper.GetString("rancher.bootstrap_password")

	CreateInstallScript(bootstrapPassword, viper.GetString("ha-1.image"), viper.GetString("ha-1.chart"), 1)
	CreateCertManagerInstallScript(1)
	CreateCACertScript(1)
	CreateLBFile(infra1LB, 1)

	CreateInstallScript(bootstrapPassword, viper.GetString("ha-2.image"), viper.GetString("ha-2.chart"), 2)
	CreateCertManagerInstallScript(2)
	CreateCACertScript(2)
	CreateLBFile(infra2LB, 2)

	log.Printf("HA 1 LB: %s", infra1LB)
	log.Printf("HA 2 LB: %s", infra2LB)
}

func TestHACleanup(t *testing.T) {
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: "../modules/aws",
		NoColor:      true,
	})
	terraform.Destroy(t, terraformOptions)

	RemoveFile("./high-availability-1/cluster.yml")
	RemoveFile("./high-availability-1/install.sh")
	RemoveFile("./high-availability-1/aws_lb_1.txt")
	RemoveFile("./high-availability-1/cert-mananger.sh")
	RemoveFolder("high-availability-1")

	RemoveFile("./high-availability-2/cluster.yml")
	RemoveFile("./high-availability-2/install.sh")
	RemoveFile("./high-availability-1/aws_lb_2.txt")
	RemoveFile("./high-availability-2/cert-mananger.sh")
	RemoveFolder("high-availability-2")

	defer RemoveFolder("../modules/aws/.terraform")
	defer RemoveFile("../modules/aws/.terraform.lock.hcl")
	defer RemoveFile("../modules/aws/terraform.tfstate")
	defer RemoveFile("../modules/aws/terraform.tfstate.backup")
	defer RemoveFile("../modules/aws/terraform.tfvars")
}

func CheckIPAddress(ip string) string {
	if net.ParseIP(ip) == nil {
		return "invalid"
	} else {
		return "valid"
	}
}

func WriteRkeConfig(pemPath, ip1, ip2, ip3, ip1private, ip2private, ip3private, fileName string) {
	c1 := Config{
		SSHKeyPath: pemPath,
		Nodes: []ConfigNode{
			{
				Address:         ip1,
				InternalAddress: ip1private,
				User:            "ubuntu",
				Role:            []string{"etcd", "controlplane", "worker"},
			},
			{
				Address:         ip2,
				InternalAddress: ip2private,
				User:            "ubuntu",
				Role:            []string{"etcd", "controlplane", "worker"},
			}, {
				Address:         ip3,
				InternalAddress: ip3private,
				User:            "ubuntu",
				Role:            []string{"etcd", "controlplane", "worker"},
			},
		},
	}

	yamlData, err := yaml.Marshal(&c1)

	if err != nil {
		fmt.Printf("Error while Marshaling. %v", err)
	}

	err = os.WriteFile(fileName, yamlData, 0644)
	if err != nil {
		panic("Unable to write data into the file")
	}
}

type Config struct {
	SSHKeyPath string       `yaml:"ssh_key_path"`
	Nodes      []ConfigNode `yaml:"nodes"`
}

type ConfigNode struct {
	Address         string   `yaml:"address"`
	InternalAddress string   `yaml:"internal_address"`
	User            string   `yaml:"user"`
	Role            []string `yaml:"role"`
}

func RemoveFile(filePath string) {
	err := os.Remove(filePath)
	if err != nil {
		log.Println(err)
	}
}

func CreateDir(path string) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			log.Println(err)
		}
	}
}

func RemoveFolder(folderPath string) {
	err := os.RemoveAll(folderPath)
	if err != nil {
		log.Println(err)
	}
}

func CreateInstallScript(bsPassword, image, chart string, ha1Or2 int) {

	var path string
	var globalPspEnabled bool
	var installScript string

	if ha1Or2 == 1 {
		path = "./high-availability-1/install.sh"
		globalPspEnabled = viper.GetBool("ha-1.global_cattle_psp_enabled")
	}

	if ha1Or2 == 2 {
		path = "./high-availability-2/install.sh"
		globalPspEnabled = viper.GetBool("ha-2.global_cattle_psp_enabled")
	}

	if globalPspEnabled == true {
		installScript = `#!/bin/sh

export KUBECONFIG=kube_config_cluster.yml

helm repo update

kubectl create namespace cattle-system

helm install rancher rancher-latest/rancher \
  --namespace cattle-system \
  --set hostname="" \
  --set ingress.tls.source=letsEncrypt \
  --set letsEncrypt.email= \
  --set letsEncrypt.ingress.class=nginx \
  --set bootstrapPassword=` + bsPassword + ` \
  --set rancherImageTag=` + image + ` \
  --version ` + chart + ` \
  --set agentTLSMode=system-store \
  --set privateCA=true
`
	} else {
		installScript = `#!/bin/sh

export KUBECONFIG=kube_config_cluster.yml

helm repo update

kubectl create namespace cattle-system

helm install rancher rancher-latest/rancher \
  --namespace cattle-system \
  --set hostname="" \
  --set ingress.tls.source=letsEncrypt \
  --set letsEncrypt.email= \
  --set letsEncrypt.ingress.class=nginx \
  --set bootstrapPassword=` + bsPassword + ` \
  --set rancherImageTag=` + image + ` \
  --version ` + chart + ` \
  --set global.cattle.psp.enabled=false \
  --set agentTLSMode=system-store \
  --set privateCA=true
`
	}

	f := []byte(installScript)
	err := os.WriteFile(path, f, 0644)

	if err != nil {
		log.Println("failed creating install script:", err)
	}
}

func CreateCACertScript(ha1Or2 int) {

	var path string
	var installScript string

	if ha1Or2 == 1 {
		path = "./high-availability-1/cacert.sh"
	}

	if ha1Or2 == 2 {
		path = "./high-availability-2/ccacert.sh"
	}

	installScript = `#!/bin/sh
export KUBECONFIG=kube_config_cluster.yml

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.0/cert-manager.crds.yaml

kubectl -n cattle-system create secret generic tls-ca \
  --from-file=cacerts.pem=./cacerts.pem
`

	f := []byte(installScript)
	err := os.WriteFile(path, f, 0644)

	if err != nil {
		log.Println("failed creating cert manager install script:", err)
	}
}

func CreateCertManagerInstallScript(ha1Or2 int) {

	var path string
	var installScript string

	if ha1Or2 == 1 {
		path = "./high-availability-1/cert-mananger.sh"
	}

	if ha1Or2 == 2 {
		path = "./high-availability-2/cert-manager.sh"
	}

	installScript = `#!/bin/sh
export KUBECONFIG=kube_config_cluster.yml

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.0/cert-manager.crds.yaml

# Add the Jetstack Helm repository
helm repo add jetstack https://charts.jetstack.io

# Update your local Helm chart repository cache
helm repo update

# Install the cert-manager Helm chart
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.15.0
`

	f := []byte(installScript)
	err := os.WriteFile(path, f, 0644)

	if err != nil {
		log.Println("failed creating cert manager install script:", err)
	}
}

func CreateLBFile(theLB string, ha1Or2 int) {

	var path string

	if ha1Or2 == 1 {
		path = "./high-availability-1/aws_lb_1.txt"
	}

	if ha1Or2 == 2 {
		path = "./high-availability-2/aws_lb_2.txt"
	}

	f := []byte(theLB)
	err := os.WriteFile(path, f, 0644)

	if err != nil {
		log.Println("failed creating aws lb text file:", err)
	}
}
