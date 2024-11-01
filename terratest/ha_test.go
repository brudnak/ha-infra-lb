package test

import (
	"encoding/json"
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

type TerraformOutputs struct {
	Server1IP        string
	Server2IP        string
	Server3IP        string
	Server1PrivateIP string
	Server2PrivateIP string
	Server3PrivateIP string
	LoadBalancerDNS  string
}

func TestHaSetup(t *testing.T) {
	setupConfig(t)

	totalHAs := viper.GetInt("total_has")
	if totalHAs < 1 {
		t.Fatal("total_has must be at least 1")
	}

	terraformOptions := getTerraformOptions(t, totalHAs)
	terraform.InitAndApply(t, terraformOptions)

	outputs := getTerraformOutputs(t, terraformOptions)
	if len(outputs) == 0 {
		t.Fatal("No outputs received from terraform")
	}

	pemPath := viper.GetString("local.pem_path")
	assert.NotEmpty(t, pemPath, "PEM path cannot be empty")

	for i := 1; i <= totalHAs; i++ {
		processHAInstance(t, i, pemPath, outputs)
	}
}

func TestHACleanup(t *testing.T) {
	setupConfig(t)
	totalHAs := viper.GetInt("total_has")

	terraformOptions := getTerraformOptions(t, totalHAs)
	terraform.Destroy(t, terraformOptions)

	for i := 1; i <= totalHAs; i++ {
		cleanupHAInstance(i)
	}
	cleanupTerraformFiles()
}

func setupConfig(t *testing.T) {
	viper.AddConfigPath("../")
	viper.SetConfigName("tool-config")
	viper.SetConfigType("yml")

	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
}

func getTerraformOptions(t *testing.T, totalHAs int) *terraform.Options {
	generateAwsVars()

	return terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: "../modules/aws",
		NoColor:      true,
		Vars: map[string]interface{}{
			"total_has":             totalHAs,
			"aws_prefix":            viper.GetString("tf_vars.aws_prefix"),
			"aws_access_key":        viper.GetString("tf_vars.aws_access_key"),
			"aws_secret_key":        viper.GetString("tf_vars.aws_secret_key"),
			"aws_vpc":               viper.GetString("tf_vars.aws_vpc"),
			"aws_subnet_a":          viper.GetString("tf_vars.aws_subnet_a"),
			"aws_subnet_b":          viper.GetString("tf_vars.aws_subnet_b"),
			"aws_subnet_c":          viper.GetString("tf_vars.aws_subnet_c"),
			"aws_ami":               viper.GetString("tf_vars.aws_ami"),
			"aws_subnet_id":         viper.GetString("tf_vars.aws_subnet_id"),
			"aws_security_group_id": viper.GetString("tf_vars.aws_security_group_id"),
			"aws_pem_key_name":      viper.GetString("tf_vars.aws_pem_key_name"),
		},
	})
}

func generateAwsVars() {
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
}

func getTerraformOutputs(t *testing.T, terraformOptions *terraform.Options) map[string]string {
	output := terraform.OutputJson(t, terraformOptions, "flat_outputs")

	var outputs map[string]string
	if err := json.Unmarshal([]byte(output), &outputs); err != nil {
		t.Logf("Raw output: %s", output)
		t.Fatalf("Failed to parse terraform outputs: %v", err)
	}

	return outputs
}

func getHAOutputs(instanceNum int, outputs map[string]string) TerraformOutputs {
	prefix := fmt.Sprintf("ha_%d", instanceNum)
	return TerraformOutputs{
		Server1IP:        outputs[fmt.Sprintf("%s_server1_ip", prefix)],
		Server2IP:        outputs[fmt.Sprintf("%s_server2_ip", prefix)],
		Server3IP:        outputs[fmt.Sprintf("%s_server3_ip", prefix)],
		Server1PrivateIP: outputs[fmt.Sprintf("%s_server1_private_ip", prefix)],
		Server2PrivateIP: outputs[fmt.Sprintf("%s_server2_private_ip", prefix)],
		Server3PrivateIP: outputs[fmt.Sprintf("%s_server3_private_ip", prefix)],
		LoadBalancerDNS:  outputs[fmt.Sprintf("%s_aws_lb", prefix)],
	}
}

func processHAInstance(t *testing.T, instanceNum int, pemPath string, outputs map[string]string) {
	haDir := fmt.Sprintf("high-availability-%d", instanceNum)

	haOutputs := getHAOutputs(instanceNum, outputs)
	validateIPs(t, haOutputs)

	CreateDir(haDir)
	createConfigurations(t, haDir, pemPath, haOutputs)

	log.Printf("HA %d LB: %s", instanceNum, haOutputs.LoadBalancerDNS)
}

func validateIPs(t *testing.T, outputs TerraformOutputs) {
	ips := []string{
		outputs.Server1IP, outputs.Server2IP, outputs.Server3IP,
		outputs.Server1PrivateIP, outputs.Server2PrivateIP, outputs.Server3PrivateIP,
	}

	for _, ip := range ips {
		assert.Equal(t, "valid", CheckIPAddress(ip), fmt.Sprintf("Invalid IP address: %s", ip))
	}
}

func createConfigurations(t *testing.T, haDir, pemPath string, outputs TerraformOutputs) {
	WriteRkeConfig(
		pemPath,
		outputs.Server1IP, outputs.Server2IP, outputs.Server3IP,
		outputs.Server1PrivateIP, outputs.Server2PrivateIP, outputs.Server3PrivateIP,
		fmt.Sprintf("%s/cluster.yml", haDir),
	)

	bootstrapPassword := viper.GetString("rancher.bootstrap_password")
	haConfig := viper.Get("ha_config").(map[string]interface{})

	CreateInstallScript(bootstrapPassword, haConfig["image"].(string), haConfig["chart"].(string), haDir)
	CreateCertManagerInstallScript(haDir)
	CreateCACertScript(haDir)
	CreateLBFile(outputs.LoadBalancerDNS, haDir)
}

func cleanupHAInstance(instanceNum int) {
	haDir := fmt.Sprintf("high-availability-%d", instanceNum)

	filesToRemove := []string{
		fmt.Sprintf("%s/cluster.yml", haDir),
		fmt.Sprintf("%s/install.sh", haDir),
		fmt.Sprintf("%s/aws_lb.txt", haDir),
		fmt.Sprintf("%s/cert-manager.sh", haDir),
		fmt.Sprintf("%s/cacert.sh", haDir),
	}

	for _, file := range filesToRemove {
		RemoveFile(file)
	}

	RemoveFolder(haDir)
}

func cleanupTerraformFiles() {
	files := []string{
		"../modules/aws/.terraform.lock.hcl",
		"../modules/aws/terraform.tfstate",
		"../modules/aws/terraform.tfstate.backup",
		"../modules/aws/terraform.tfvars",
	}

	for _, file := range files {
		RemoveFile(file)
	}

	RemoveFolder("../modules/aws/.terraform")
}

func CreateInstallScript(bsPassword, image, chart, haDir string) {
	installScript := fmt.Sprintf(`#!/bin/sh
export KUBECONFIG=kube_config_cluster.yml

helm repo update

kubectl create namespace cattle-system

helm install rancher rancher-latest/rancher \
  --namespace cattle-system \
  --set hostname="" \
  --set ingress.tls.source=letsEncrypt \
  --set letsEncrypt.email= \
  --set letsEncrypt.ingress.class=nginx \
  --set bootstrapPassword=%s \
  --set rancherImageTag=%s \
  --version %s \
  --set agentTLSMode=system-store \
  --set privateCA=true`, bsPassword, image, chart)

	writeFile(fmt.Sprintf("%s/install.sh", haDir), []byte(installScript))
}

func CreateCertManagerInstallScript(haDir string) {
	installScript := `#!/bin/sh
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
  --version v1.15.0`

	writeFile(fmt.Sprintf("%s/cert-manager.sh", haDir), []byte(installScript))
}

func CreateCACertScript(haDir string) {
	installScript := `#!/bin/sh
export KUBECONFIG=kube_config_cluster.yml

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.0/cert-manager.crds.yaml

kubectl -n cattle-system create secret generic tls-ca \
  --from-file=cacerts.pem=./cacerts.pem`

	writeFile(fmt.Sprintf("%s/cacert.sh", haDir), []byte(installScript))
}

func CreateLBFile(theLB, haDir string) {
	writeFile(fmt.Sprintf("%s/aws_lb.txt", haDir), []byte(theLB))
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
			},
			{
				Address:         ip3,
				InternalAddress: ip3private,
				User:            "ubuntu",
				Role:            []string{"etcd", "controlplane", "worker"},
			},
		},
	}

	yamlData, err := yaml.Marshal(&c1)
	if err != nil {
		log.Printf("Error while marshaling RKE config: %v", err)
		return
	}

	writeFile(fileName, yamlData)
}

func writeFile(path string, data []byte) {
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("Failed to write file %s: %v", path, err)
	}
}

func CheckIPAddress(ip string) string {
	if net.ParseIP(ip) == nil {
		return "invalid"
	}
	return "valid"
}

func RemoveFile(filePath string) {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to remove file %s: %v", filePath, err)
	}
}

func CreateDir(path string) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, os.ModePerm); err != nil {
			log.Printf("Failed to create directory %s: %v", path, err)
		}
	}
}

func RemoveFolder(folderPath string) {
	if err := os.RemoveAll(folderPath); err != nil {
		log.Printf("Failed to remove folder %s: %v", folderPath, err)
	}
}
