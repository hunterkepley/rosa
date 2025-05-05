package handler

import (
	"fmt"

	"github.com/openshift-online/ocm-common/pkg/aws/aws_client"
	"github.com/openshift-online/ocm-common/pkg/test/vpc_client"

	"github.com/openshift/rosa/tests/ci/config"
	"github.com/openshift/rosa/tests/utils/exec/rosacli"
	"github.com/openshift/rosa/tests/utils/helper"
	"github.com/openshift/rosa/tests/utils/log"
)

type ResourcesHandler interface {
	DestroyResources() (errors []error)

	GetAccountRolesPrefix() string
	GetAdditionalPrincipals() string
	GetAuditLogArn() string
	GetDNSDomain() string
	GetEtcdKMSKey() string
	GetIngressHostedZoneID() string
	GetHostedCPInternalHostedZoneID() string
	GetKMSKey() string
	GetOIDCConfigID() string
	GetOperatorRolesPrefix() string
	GetResourceShareArn() string
	GetSharedVPCRole() string
	GetVpcID() string

	GetVPC() *vpc_client.VPC
	GetAWSClient(useSharedVPCIfAvailable bool) (*aws_client.AWSClient, error)

	PrepareVersion(versionRequirement string, channelGroup string, hcp bool) (*rosacli.OpenShiftVersionTableOutput, error)
	PreparePrefix(profilePrefix string, nameLength int) string
	PrepareVPC(vpcName string, cidrValue string, useExisting bool, withSharedAccount bool) (*vpc_client.VPC, error)
	PrepareSubnets(zones []string, multiZone bool) (map[string][]string, error)
	PrepareProxy(zone string, sshPemFileName string, sshPemFileRecordDir string, caFile string) (*ProxyDetail, error)
	PrepareKMSKey(multiRegion bool, testClient string, hcp bool, etcdKMS bool) (string, error)
	PrepareAdditionalSecurityGroups(securityGroupCount int, namePrefix string) ([]string, error)
	PrepareAccountRoles(namePrefix string, hcp bool, openshiftVersion string,
		channelGroup string, path string, permissionsBoundary string, route53RoleARN string,
		vpcEndpointRoleArn string) (accRoles *rosacli.AccountRolesUnit, err error)
	PrepareOperatorRolesByOIDCConfig(
		namePrefix string,
		oidcConfigID string,
		roleArn string,
		sharedRoute53RoleArn string,
		sharedVPCEndPointRoleArn string,
		hcp bool, channelGroup string) error
	PrepareAdminUser() (string, string)
	PrepareAuditlogRoleArnByOIDCConfig(auditLogRoleName string, oidcConfigID string) (string, error)
	PrepareAuditlogRoleArnByIssuer(auditLogRoleName string, oidcIssuerURL string) (string, error)
	PrepareOperatorRolesByCluster(clusterID string) error
	PrepareOIDCConfig(oidcConfigType string, roleArn string, prefix string) (string, error)
	PrepareOIDCProvider(oidcConfigID string) error
	PrepareOIDCProviderByCluster(clusterID string) error
	PrepareSharedVPCRole(sharedVPCRolePrefix string, installerRoleArn string,
		ingressOperatorRoleArn string) (string, string, error)
	PrepareAdditionalPrincipalsRole(roleName string, installerRoleArn string) (string, error)
	PrepareDNSDomain(hostedcp bool) (string, error)
	PrepareHostedZone(hostedZoneName string, vpcID string, private bool) (string, error)
	PrepareSubnetArns(subnetIDs string) ([]string, error)
	PrepareResourceShare(resourceShareName string, resourceArns []string) (string, error)

	DeleteVPCChain(withSharedAccount bool) error
	DeleteKMSKey(etcdKMS bool) error
	DeleteAuditLogRoleArn() error
	DeleteHostedZone(hostedZoneID string) error
	DeleteDNSDomain() error
	DeleteSharedVPCRole(managedPolicy bool) error
	DeleteAdditionalPrincipalsRole(managedPolicy bool) error
	DeleteResourceShare() error
	DeleteOperatorRoles() error
	DeleteOIDCConfig() error
	DeleteAccountRoles() error
}

type resourcesHandler struct {
	resources                       *Resources
	persist                         bool
	rosaClient                      *rosacli.Client
	awsCredentialsFile              string
	awsSharedAccountCredentialsFile string

	// Optional
	vpc *vpc_client.VPC
}

// NewResourcesHandler create a new resources handler with data persisted to Filesystem
func NewResourcesHandler(client *rosacli.Client, region string,
	awsCredentialsFile string, awsSharedAccountCredentialsFile string) (ResourcesHandler, error) {

	return newResourcesHandler(client, region, true, false, awsCredentialsFile, awsSharedAccountCredentialsFile)
}

// NewTempResourcesHandler create a new resources handler WITHOUT data written to Filesystem
// Useful for test cases needed resources. Do not forget to delete the resources afterwards
// awsSharedAccountCredentialsFile is the second AWS account for shared resources
func NewTempResourcesHandler(client *rosacli.Client, region string,
	awsCredentialsFile string, awsSharedAccountCredentialsFile string) (ResourcesHandler, error) {

	return newResourcesHandler(client, region, false, false, awsCredentialsFile, awsSharedAccountCredentialsFile)
}

// NewResourcesHandlerFromFilesystem create a new resources handler from data saved on Filesystem
func NewResourcesHandlerFromFilesystem(client *rosacli.Client,
	awsCredentialsFile string, awsSharedAccountCredentialsFile string) (ResourcesHandler, error) {

	return newResourcesHandler(client, "", true, true, awsCredentialsFile, awsSharedAccountCredentialsFile)
}

func newResourcesHandler(client *rosacli.Client, region string, persist bool,
	loadFilesystem bool, awsCredentialsFile string,
	awsSharedAccountCredentialsFile string) (*resourcesHandler, error) {

	resourcesHandler := &resourcesHandler{
		rosaClient:                      client,
		resources:                       &Resources{Region: region},
		persist:                         persist,
		awsCredentialsFile:              awsCredentialsFile,
		awsSharedAccountCredentialsFile: awsSharedAccountCredentialsFile,
	}

	if loadFilesystem {
		err := helper.ReadFileContentToObject(config.Test.UserDataFile, &resourcesHandler.resources)
		if err != nil {
			log.Logger.Errorf("Error happened when parse resource file data to UserData struct: %s", err.Error())
			return nil, err
		}
	}

	return resourcesHandler, nil
}

func (rh *resourcesHandler) DestroyResources() (errors []error) {
	var err error
	resources := rh.resources

	defer func() {
		log.Logger.Info("Rewrite User data file")
		rh.saveToFile()
	}()

	destroyLog := func(err error, resource string) bool {
		if err != nil {
			log.Logger.Errorf("Error happened when delete %s: %s", resource, err.Error())
			errors = append(errors, err)
			return false
		}
		log.Logger.Infof("Delete %s successfully", resource)
		return true
	}

	// schedule KMS key
	if resources.KMSKey != "" {
		log.Logger.Infof("Find prepared kms key: %s. Going to schedule the deletion.", resources.KMSKey)
		err = rh.DeleteKMSKey(false)
		success := destroyLog(err, "kms key")
		if success {
			rh.registerKMSKey("")
		}
	}
	// schedule Etcd KMS key
	if resources.EtcdKMSKey != "" {
		log.Logger.Infof("Find prepared etcd kms key: %s. Going to schedule the deletion", resources.EtcdKMSKey)
		err = rh.DeleteKMSKey(true)
		success := destroyLog(err, "etcd kms key")
		if success {
			rh.registerEtcdKMSKey("")
		}
	}
	// delete audit log arn
	if resources.AuditLogArn != "" {
		log.Logger.Infof("Find prepared audit log arn: %s", resources.AuditLogArn)
		err = rh.DeleteAuditLogRoleArn()
		success := destroyLog(err, "audit log arn")
		if success {
			rh.registerAuditLogArn("")
		}
	}
	//delete hosted zones
	if resources.IngressHostedZoneID != "" {
		log.Logger.Infof("Find prepared ingress hosted zone: %s", resources.IngressHostedZoneID)
		err = rh.DeleteHostedZone(resources.IngressHostedZoneID)
		success := destroyLog(err, "ingress hosted zone")
		if success {
			rh.registerIngressHostedZoneID("")
		}
	}

	if resources.HostedCPInternalHostedZoneID != "" {
		log.Logger.Infof("Find prepared hostedcp internal hosted zone: %s", resources.HostedCPInternalHostedZoneID)
		err = rh.DeleteHostedZone(resources.HostedCPInternalHostedZoneID)
		success := destroyLog(err, "hostedcp internal hosted zone")
		if success {
			rh.registerHostedCPInternalHostedZoneID("")
		}
	}

	//delete dns domain
	if resources.DNSDomain != "" {
		log.Logger.Infof("Find prepared DNS Domain: %s", resources.DNSDomain)
		err = rh.DeleteDNSDomain()
		success := destroyLog(err, "dns domain")
		if success {
			rh.registerDNSDomain("")
		}
	}
	// Delete proxy resourses
	if resources.ProxyInstanceID != "" {
		err = rh.CleanupProxyResources(
			resources.ProxyInstanceID,
			resources.FromSharedAWSAccount != nil && resources.FromSharedAWSAccount.VPC,
		)
		success := destroyLog(err, "proxy resources")
		if success {
			rh.registerProxyInstanceID("")
		}
	}
	// delete resource share
	if resources.ResourceShareArn != "" {
		log.Logger.Infof("Find prepared resource share: %s", resources.ResourceShareArn)
		err = rh.DeleteResourceShare()
		success := destroyLog(err, "resource share")
		if success {
			rh.registerResourceShareArn("")
		}
	}
	// delete vpc chain
	if resources.VpcID != "" {
		log.Logger.Infof("Find prepared vpc id: %s", resources.VpcID)
		err = rh.DeleteVPCChain(resources.FromSharedAWSAccount != nil && resources.FromSharedAWSAccount.VPC)
		success := destroyLog(err, "vpc chain")
		if success {
			rh.registerVpcID("", false)
		}
	}
	// delete shared vpc role -- for classic shared vpc cluster
	if resources.SharedVPCRole != "" {
		log.Logger.Infof("Find prepared classic shared vpc role: %s", resources.SharedVPCRole)
		err = rh.DeleteSharedVPCRole(false)
		success := destroyLog(err, "classic shared vpc role")
		if success {
			rh.registerSharedVPCRole("")
		}
	}

	// delete shared vpc role -- for hosted-cp shared vpc cluster
	if resources.HCPRoute53ShareRole != "" || resources.HCPVPCEndpointShareRole != "" {
		log.Logger.Infof("Find prepared hostedcp shared route53 and vpc endpoint roles: %s and %s",
			resources.HCPRoute53ShareRole, resources.HCPVPCEndpointShareRole)
		err = rh.DeleteHostedCPSharedVPCRoles(false)
		success := destroyLog(err, "hostedcp shared vpc roles(shared route53 role and shared vpc endpoint role)")
		if success {
			rh.registerSharedRoute53Role("")
			rh.registerSharedVPCEndpointRole("")
		}
	}

	// delete additional principal role
	if resources.AdditionalPrincipals != "" {
		log.Logger.Infof("Find prepared additional principal role: %s", resources.AdditionalPrincipals)
		err = rh.DeleteAdditionalPrincipalsRole(true)
		success := destroyLog(err, "additional principal role")
		if success {
			rh.registerAdditionalPrincipals("")
		}
	}
	// delete operator roles
	if resources.OperatorRolesPrefix != "" {
		log.Logger.Infof("Find prepared operator roles with prefix: %s", resources.OperatorRolesPrefix)
		err = rh.DeleteOperatorRoles()
		success := destroyLog(err, "operator roles")
		if success {
			rh.registerOperatorRolesPrefix("")
		}
	}
	// delete oidc config
	if resources.OIDCConfigID != "" {
		log.Logger.Infof("Find prepared oidc config id: %s", resources.OIDCConfigID)
		err = rh.DeleteOIDCConfig()
		success := destroyLog(err, "oidc config")
		if success {
			rh.registerOIDCConfigID("")
		}
	}
	// delete account roles
	if resources.AccountRolesPrefix != "" {
		log.Logger.Infof("Find prepared account roles with prefix: %s", resources.AccountRolesPrefix)
		err = rh.DeleteAccountRoles()
		success := destroyLog(err, "account roles")
		if success {
			rh.registerAccountRolesPrefix("")
		}
	}

	if len(errors) <= 0 {
		rh.resources = &Resources{}
		rh.saveToFile()
	}

	return errors
}

func (rh *resourcesHandler) saveToFile() (err error) {
	if !rh.persist {
		log.Logger.Debug("Ignoring save to file as per configuration")
		return
	}
	_, err = helper.CreateFileWithContent(config.Test.UserDataFile, &rh.resources)
	if err != nil {
		return
	}
	if rh.vpc != nil {
		_, err = helper.CreateFileWithContent(config.Test.VPCIDFile, rh.vpc.VpcID)
		if err != nil {
			return
		}
		if len(rh.vpc.AllPublicSubnetIDs()) != 0 {
			_, err = helper.CreateFileWithContent(config.Test.PublicSubnetsFile,
				fmt.Sprintf(`['%s']`, rh.vpc.AllPublicSubnetIDs()[0]))
		}
	}
	return
}

func (rh *resourcesHandler) GetAccountRolesPrefix() string {
	return rh.resources.AccountRolesPrefix
}

func (rh *resourcesHandler) GetAdditionalPrincipals() string {
	return rh.resources.AdditionalPrincipals
}

func (rh *resourcesHandler) GetAuditLogArn() string {
	return rh.resources.AuditLogArn
}

func (rh *resourcesHandler) GetDNSDomain() string {
	return rh.resources.DNSDomain
}

func (rh *resourcesHandler) GetEtcdKMSKey() string {
	return rh.resources.EtcdKMSKey
}

func (rh *resourcesHandler) GetIngressHostedZoneID() string {
	return rh.resources.IngressHostedZoneID
}
func (rh *resourcesHandler) GetHostedCPInternalHostedZoneID() string {
	return rh.resources.HostedCPInternalHostedZoneID
}

func (rh *resourcesHandler) GetKMSKey() string {
	return rh.resources.KMSKey
}

func (rh *resourcesHandler) GetOIDCConfigID() string {
	return rh.resources.OIDCConfigID
}

func (rh *resourcesHandler) GetOperatorRolesPrefix() string {
	return rh.resources.OperatorRolesPrefix
}

func (rh *resourcesHandler) GetResourceShareArn() string {
	return rh.resources.ResourceShareArn
}

func (rh *resourcesHandler) GetSharedVPCRole() string {
	return rh.resources.SharedVPCRole
}

func (rh *resourcesHandler) GetVpcID() string {
	return rh.resources.VpcID
}

func (rh *resourcesHandler) registerAccountRolesPrefix(accountRolesPrefix string) error {
	rh.resources.AccountRolesPrefix = accountRolesPrefix
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerAdditionalPrincipals(additionalPrincipals string) error {
	rh.resources.AdditionalPrincipals = additionalPrincipals
	if rh.resources.FromSharedAWSAccount == nil {
		rh.resources.FromSharedAWSAccount = &FromSharedAWSAccount{}
	}
	rh.resources.FromSharedAWSAccount.AdditionalPrincipls = true
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerAuditLogArn(auditLogArn string) error {
	rh.resources.AuditLogArn = auditLogArn
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerDNSDomain(dnsDomain string) error {
	rh.resources.DNSDomain = dnsDomain
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerEtcdKMSKey(etcdKMSKey string) error {
	rh.resources.EtcdKMSKey = etcdKMSKey
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerIngressHostedZoneID(hostedZoneID string) error {
	rh.resources.IngressHostedZoneID = hostedZoneID
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerHostedCPInternalHostedZoneID(hostedZoneID string) error {
	rh.resources.HostedCPInternalHostedZoneID = hostedZoneID
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerKMSKey(kmsKey string) error {
	rh.resources.KMSKey = kmsKey
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerOIDCConfigID(oidcConfigID string) error {
	rh.resources.OIDCConfigID = oidcConfigID
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerOperatorRolesPrefix(operatorRolesPrefix string) error {
	rh.resources.OperatorRolesPrefix = operatorRolesPrefix
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerResourceShareArn(resourceShareArn string) error {
	rh.resources.ResourceShareArn = resourceShareArn
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerSharedVPCRole(sharedVPCRole string) error {
	rh.resources.SharedVPCRole = sharedVPCRole
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerSharedRoute53Role(sharedVPCRoute53Role string) error {
	rh.resources.HCPRoute53ShareRole = sharedVPCRoute53Role
	return rh.saveToFile()
}
func (rh *resourcesHandler) registerSharedVPCEndpointRole(sharedVPCEndpointRole string) error {
	rh.resources.HCPVPCEndpointShareRole = sharedVPCEndpointRole
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerVpcID(vpcID string, fromSharedAccount bool) error {
	rh.resources.VpcID = vpcID
	if rh.resources.FromSharedAWSAccount == nil {
		rh.resources.FromSharedAWSAccount = &FromSharedAWSAccount{}
	}
	rh.resources.FromSharedAWSAccount.VPC = fromSharedAccount
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerVPC(vpc *vpc_client.VPC) error {
	rh.vpc = vpc
	return rh.saveToFile()
}

func (rh *resourcesHandler) registerProxyInstanceID(proxyInsID string) error {
	rh.resources.ProxyInstanceID = proxyInsID
	return rh.saveToFile()
}

func (rh *resourcesHandler) GetVPC() *vpc_client.VPC {
	return rh.vpc
}

func (rh *resourcesHandler) GetAWSClient(useSharedAccount bool) (*aws_client.AWSClient, error) {
	if useSharedAccount {
		if rh.awsSharedAccountCredentialsFile == "" {
			return nil, fmt.Errorf("the shared aws account credential file is empty. Please set it by export ")
		}
		return aws_client.CreateAWSClient("", rh.resources.Region, rh.awsSharedAccountCredentialsFile)
	}
	return aws_client.CreateAWSClient("", rh.resources.Region)
}
