package context

import (
	"fmt"
	"math"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	//20210608 added ausf context
	"regexp"
	"strconv"

	"github.com/google/uuid"

	"free5gc/lib/idgenerator"
	"free5gc/lib/openapi"
	"free5gc/lib/openapi/Nnrf_NFDiscovery"
	"free5gc/lib/openapi/Nnrf_NFManagement"
	"free5gc/lib/openapi/Nudm_SubscriberDataManagement"
	"free5gc/lib/openapi/models"
	"free5gc/lib/pfcp/pfcpType"
	"free5gc/lib/pfcp/pfcpUdp"
	"free5gc/src/smpcf/factory"
	"free5gc/src/smpcf/logger"
)

func init() {
	smpcfContext.NfInstanceID = uuid.New().String()
	SMPCF_Self().PcfServiceUris = make(map[models.ServiceName]string)
	SMPCF_Self().PcfSuppFeats = make(map[models.ServiceName]openapi.SupportedFeature)
	SMPCF_Self().AMFStatusSubsData = make(map[string]AMFStatusSubscriptionData)
}

var smpcfContext SMPCFContext

type SMPCFContext struct {
	Name         string
	NfInstanceID string

	UriScheme    models.UriScheme
	BindingIPv4  string
	RegisterIPv4 string
	SBIPort      int
	CPNodeID     pfcpType.NodeID

	UDMProfile models.NfProfile

	SnssaiInfos []models.SnssaiSmfInfoItem

	UPNodeIDs []pfcpType.NodeID
	Key       string
	PEM       string
	KeyLog    string

	UESubNet      *net.IPNet
	UEAddressTemp net.IP
	UEAddressLock sync.Mutex

	NrfUri                         string
	NFManagementClient             *Nnrf_NFManagement.APIClient
	NFDiscoveryClient              *Nnrf_NFDiscovery.APIClient
	SubscriberDataManagementClient *Nudm_SubscriberDataManagement.APIClient
	DNNInfo                        map[string]factory.DNNInfo

	UserPlaneInformation *UserPlaneInformation
	OnlySupportIPv4      bool
	OnlySupportIPv6      bool
	//*** For ULCL ** //
	ULCLSupport         bool
	UEPreConfigPathPool map[string]*UEPreConfigPaths
	LocalSEIDCount      uint64

	//20210608 added from AUSFContext
	suciSupiMap sync.Map
	UePool      sync.Map
	//NfId        string
	GroupID    string
	Url        string
	NfService  map[models.ServiceName]models.NfService
	PlmnList   []models.PlmnId
	UdmUeauUrl string
	snRegex    *regexp.Regexp
	//20210618 added from PCFContext
	TimeFormat      string
	DefaultBdtRefId string
	PcfServiceUris  map[models.ServiceName]string
	PcfSuppFeats    map[models.ServiceName]openapi.SupportedFeature
	DefaultUdrURI   string
	// Bdt Policy related
	BdtPolicyPool        sync.Map
	BdtPolicyIDGenerator *idgenerator.IDGenerator
	// App Session related
	AppSessionPool sync.Map
	// AMF Status Change Subscription related
	AMFStatusSubsData map[string]AMFStatusSubscriptionData // subscriptionId as key
	//lock
	DefaultUdrURILock sync.RWMutex
}

//20210610 added from ausf_context.go
type AusfUeContext struct {
	Supi               string
	Kausf              string
	Kseaf              string
	ServingNetworkName string
	AuthStatus         models.AuthResult
	UdmUeauUrl         string

	// for 5G AKA
	XresStar string

	// for EAP-AKA'
	K_aut string
	XRES  string
	Rand  string
}

type SuciSupiMap struct {
	SupiOrSuci string
	Supi       string
}

const (
	EAP_AKA_PRIME_TYPENUM = 50
)

// Attribute Types for EAP-AKA'
const (
	AT_RAND_ATTRIBUTE         = 1
	AT_AUTN_ATTRIBUTE         = 2
	AT_RES_ATTRIBUTE          = 3
	AT_MAC_ATTRIBUTE          = 11
	AT_NOTIFICATION_ATTRIBUTE = 12
	AT_IDENTITY_ATTRIBUTE     = 14
	AT_KDF_INPUT_ATTRIBUTE    = 23
	AT_KDF_ATTRIBUTE          = 24
)

//20210618 added from pcf_context.go
type AMFStatusSubscriptionData struct {
	AmfUri string

	AmfStatusUri string

	GuamiList []models.Guami
}
type AppSessionData struct {
	AppSessionId      string
	AppSessionContext *models.AppSessionContext
	// (compN/compN-subCompN/appId-%s) map to PccRule
	RelatedPccRuleIds    map[string]string
	PccRuleIdMapToCompId map[string]string
	// EventSubscription
	Events   map[models.AfEvent]models.AfNotifMethod
	EventUri string
	// related Session
	SmPolicyData *UeSmPolicyData
}

func AllocUEIP() net.IP {
	smpcfContext.UEAddressLock.Lock()
	defer smpcfContext.UEAddressLock.Unlock()
	smpcfContext.UEAddressTemp[3]++
	return smpcfContext.UEAddressTemp
}

func AllocateLocalSEID() uint64 {
	atomic.AddUint64(&smpcfContext.LocalSEIDCount, 1)
	return smpcfContext.LocalSEIDCount
}

func InitSmpcfContext(config *factory.Config) {
	if config == nil {
		logger.SMPCFContextLog.Error("Config is nil")
		return
	}
	//20210610 added
	snRegex, err := regexp.Compile("5G:mnc[0-9]{3}[.]mcc[0-9]{3}[.]3gppnetwork[.]org")
	if err != nil {
		logger.SMPCFContextLog.Warnf("SN compile error: %+v", err)
	} else {
		smpcfContext.snRegex = snRegex
	}
	//
	logger.SMPCFContextLog.Infof("SmpcfConfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)
	configuration := config.Configuration
	if configuration.SmpcfName != "" {
		smpcfContext.Name = configuration.SmpcfName
	}

	sbi := configuration.Sbi
	if sbi == nil {
		logger.SMPCFContextLog.Errorln("Configuration needs \"sbi\" value")
		return
	} else {
		smpcfContext.UriScheme = models.UriScheme(sbi.Scheme)
		smpcfContext.RegisterIPv4 = "127.0.0.1" // default localhost
		smpcfContext.SBIPort = 29502            // default port
		if sbi.RegisterIPv4 != "" {
			smpcfContext.RegisterIPv4 = sbi.RegisterIPv4
		}
		if sbi.Port != 0 {
			smpcfContext.SBIPort = sbi.Port
		}

		if tls := sbi.TLS; tls != nil {
			smpcfContext.Key = tls.Key
			smpcfContext.PEM = tls.PEM
		}

		smpcfContext.BindingIPv4 = os.Getenv(sbi.BindingIPv4)
		if smpcfContext.BindingIPv4 != "" {
			logger.SMPCFContextLog.Info("Parsing ServerIPv4 address from ENV Variable.")
		} else {
			smpcfContext.BindingIPv4 = sbi.BindingIPv4
			if smpcfContext.BindingIPv4 == "" {
				logger.SMPCFContextLog.Warn("Error parsing ServerIPv4 address as string. Using the 0.0.0.0 address as default.")
				smpcfContext.BindingIPv4 = "0.0.0.0"
			}
		}
	}
	//20210608 added ausf context initialize
	//smpcfContext.NfId = smpcfContext.NfInstanceID
	smpcfContext.Url = string(smpcfContext.UriScheme) + "://" + smpcfContext.RegisterIPv4 + ":" + strconv.Itoa(smpcfContext.SBIPort)
	smpcfContext.PlmnList = append(smpcfContext.PlmnList, configuration.PlmnSupportList...)

	if configuration.NrfUri != "" {
		smpcfContext.NrfUri = configuration.NrfUri
	} else {
		logger.SMPCFContextLog.Warn("NRF Uri is empty! Using localhost as NRF IPv4 address.")
		smpcfContext.NrfUri = fmt.Sprintf("%s://%s:%d", smpcfContext.UriScheme, "127.0.0.1", 29510)
	}

	if pfcp := configuration.PFCP; pfcp != nil {
		if pfcp.Port == 0 {
			pfcp.Port = pfcpUdp.PFCP_PORT
		}
		pfcpAddrEnv := os.Getenv(pfcp.Addr)
		if pfcpAddrEnv != "" {
			logger.SMPCFContextLog.Info("Parsing PFCP IPv4 address from ENV variable found.")
			pfcp.Addr = pfcpAddrEnv
		}
		if pfcp.Addr == "" {
			logger.SMPCFContextLog.Warn("Error parsing PFCP IPv4 address as string. Using the 0.0.0.0 address as default.")
			pfcp.Addr = "0.0.0.0"
		}
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", pfcp.Addr, pfcp.Port))
		if err != nil {
			logger.SMPCFContextLog.Warnf("PFCP Parse Addr Fail: %v", err)
		}

		smpcfContext.CPNodeID.NodeIdType = 0
		smpcfContext.CPNodeID.NodeIdValue = addr.IP.To4()
	}

	_, ipNet, err := net.ParseCIDR(configuration.UESubnet)
	if err != nil {
		logger.InitLog.Errorln(err)
	}

	smpcfContext.DNNInfo = configuration.DNN
	smpcfContext.UESubNet = ipNet
	smpcfContext.UEAddressTemp = ipNet.IP

	// Set client and set url
	ManagementConfig := Nnrf_NFManagement.NewConfiguration()
	ManagementConfig.SetBasePath(SMPCF_Self().NrfUri)
	smpcfContext.NFManagementClient = Nnrf_NFManagement.NewAPIClient(ManagementConfig)

	NFDiscovryConfig := Nnrf_NFDiscovery.NewConfiguration()
	NFDiscovryConfig.SetBasePath(SMPCF_Self().NrfUri)
	smpcfContext.NFDiscoveryClient = Nnrf_NFDiscovery.NewAPIClient(NFDiscovryConfig)

	smpcfContext.ULCLSupport = configuration.ULCL

	smpcfContext.SnssaiInfos = configuration.SNssaiInfo

	smpcfContext.OnlySupportIPv4 = true

	smpcfContext.UserPlaneInformation = NewUserPlaneInformation(&configuration.UserPlaneInformation)

	SetupNFProfile(config)
}

func InitSMFUERouting(routingConfig *factory.RoutingConfig) {

	if !smpcfContext.ULCLSupport {
		return
	}

	if routingConfig == nil {
		logger.SMPCFContextLog.Error("configuration needs the routing config")
		return
	}

	logger.SMPCFContextLog.Infof("ue routing config Info: Version[%s] Description[%s]",
		routingConfig.Info.Version, routingConfig.Info.Description)

	UERoutingInfo := routingConfig.UERoutingInfo
	smpcfContext.UEPreConfigPathPool = make(map[string]*UEPreConfigPaths)

	for _, routingInfo := range UERoutingInfo {
		supi := routingInfo.SUPI
		uePreConfigPaths, err := NewUEPreConfigPaths(supi, routingInfo.PathList)
		if err != nil {
			logger.SMPCFContextLog.Warnln(err)
			continue
		}

		smpcfContext.UEPreConfigPathPool[supi] = uePreConfigPaths
	}

}

//// Create new SMPCF context
func SMPCF_Self() *SMPCFContext {
	return &smpcfContext
}

func GetUserPlaneInformation() *UserPlaneInformation {
	return smpcfContext.UserPlaneInformation
}

func NewAusfUeContext(identifier string) (ausfUeContext *AusfUeContext) {
	ausfUeContext = new(AusfUeContext)
	ausfUeContext.Supi = identifier // supi
	return ausfUeContext
}

func AddAusfUeContextToPool(ausfUeContext *AusfUeContext) {
	smpcfContext.UePool.Store(ausfUeContext.Supi, ausfUeContext)
}

func CheckIfAusfUeContextExists(ref string) bool {
	_, ok := smpcfContext.UePool.Load(ref)
	return ok
}

func GetAusfUeContext(ref string) *AusfUeContext {
	context, _ := smpcfContext.UePool.Load(ref)
	ausfUeContext := context.(*AusfUeContext)
	return ausfUeContext
}

func AddSuciSupiPairToMap(supiOrSuci string, supi string) {
	newPair := new(SuciSupiMap)
	newPair.SupiOrSuci = supiOrSuci
	newPair.Supi = supi
	smpcfContext.suciSupiMap.Store(supiOrSuci, newPair)
}

func CheckIfSuciSupiPairExists(ref string) bool {
	_, ok := smpcfContext.suciSupiMap.Load(ref)
	return ok
}

func GetSupiFromSuciSupiMap(ref string) (supi string) {
	val, _ := smpcfContext.suciSupiMap.Load(ref)
	suciSupiMap := val.(*SuciSupiMap)
	supi = suciSupiMap.Supi
	return supi
}

func IsServingNetworkAuthorized(lookup string) bool {
	fmt.Println("in smpcf IsServingNetworkAuthorized")
	if smpcfContext.snRegex.MatchString(lookup) {
		return true
	} else {
		return false
	}
}

/* replaced by SMPCF_Self()
func AUSF_Self() *AUSFContext {
	return &smpcfContext
}
*/
/* replaced by SMPCF_SelfID()
func (a *AUSFContext) AUSF_SelfID() string {
	return a.NfId
}
*/
func (a *SMPCFContext) SMPCF_SelfID() string {
	return a.NfInstanceID
}

// 20210618 added from pcf_context.go
func GetUri(name models.ServiceName) string {
	return smpcfContext.PcfServiceUris[name]
}
func (context *SMPCFContext) GetIPv4Uri() string {
	return fmt.Sprintf("%s://%s:%d", context.UriScheme, context.RegisterIPv4, context.SBIPort)
}

//20210619 added from pcf_context.go
//SetDefaultUdrURI ... function to set DefaultUdrURI
func (context *SMPCFContext) SetDefaultUdrURI(uri string) {
	context.DefaultUdrURILock.Lock()
	defer context.DefaultUdrURILock.Unlock()
	context.DefaultUdrURI = uri
}

// Find PcfUe which the policyId belongs to
func (context *SMPCFContext) PCFUeFindByPolicyId(PolicyId string) *PCFUeContext {
	index := strings.LastIndex(PolicyId, "-")
	if index == -1 {
		return nil
	}
	supi := PolicyId[:index]
	if supi != "" {
		if value, ok := context.UePool.Load(supi); ok {
			ueContext := value.(*PCFUeContext)
			return ueContext
		}
	}
	return nil
}

// Allocate PCF Ue with supi and add to pcf Context and returns allocated ue
func (context *SMPCFContext) NewPCFUe(Supi string) (*PCFUeContext, error) {
	if strings.HasPrefix(Supi, "imsi-") {
		newPCFUeContext := &PCFUeContext{}
		newPCFUeContext.SmPolicyData = make(map[string]*UeSmPolicyData)
		newPCFUeContext.AMPolicyData = make(map[string]*UeAMPolicyData)
		newPCFUeContext.PolAssociationIDGenerator = 1
		newPCFUeContext.AppSessionIDGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
		newPCFUeContext.Supi = Supi
		context.UePool.Store(Supi, newPCFUeContext)
		return newPCFUeContext, nil
	} else {
		return nil, fmt.Errorf(" add Ue context fail ")
	}
}

// Find SMPolicy with AppSessionContext
func ueSMPolicyFindByAppSessionContext(ue *PCFUeContext, req *models.AppSessionContextReqData) (*UeSmPolicyData, error) {
	var policy *UeSmPolicyData
	var err error

	if req.UeIpv4 != "" {
		policy = ue.SMPolicyFindByIdentifiersIpv4(req.UeIpv4, req.SliceInfo, req.Dnn, req.IpDomain)
		if policy == nil {
			err = fmt.Errorf("Can't find Ue with Ipv4[%s]", req.UeIpv4)
		}
	} else if req.UeIpv6 != "" {
		policy = ue.SMPolicyFindByIdentifiersIpv6(req.UeIpv6, req.SliceInfo, req.Dnn)
		if policy == nil {
			err = fmt.Errorf("Can't find Ue with Ipv6 prefix[%s]", req.UeIpv6)
		}
	} else {
		//TODO: find by MAC address
		err = fmt.Errorf("Ue finding by MAC address does not support")
	}
	return policy, err
}

// SessionBinding from application request to get corresponding Sm policy
func (context *SMPCFContext) SessionBinding(req *models.AppSessionContextReqData) (*UeSmPolicyData, error) {
	var selectedUE *PCFUeContext
	var policy *UeSmPolicyData
	var err error

	if req.Supi != "" {
		if val, exist := context.UePool.Load(req.Supi); exist {
			selectedUE = val.(*PCFUeContext)
		}
	}

	if req.Gpsi != "" && selectedUE == nil {
		context.UePool.Range(func(key, value interface{}) bool {
			ue := value.(*PCFUeContext)
			if ue.Gpsi == req.Gpsi {
				selectedUE = ue
				return false
			} else {
				return true
			}
		})
	}

	if selectedUE != nil {
		policy, err = ueSMPolicyFindByAppSessionContext(selectedUE, req)
	} else {
		context.UePool.Range(func(key, value interface{}) bool {
			ue := value.(*PCFUeContext)
			policy, err = ueSMPolicyFindByAppSessionContext(ue, req)
			return true
		})
	}
	if policy == nil && err == nil {
		err = fmt.Errorf("No SM policy found")
	}
	return policy, err
}
