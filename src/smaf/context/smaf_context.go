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
	"free5gc/src/smaf/factory"
	"free5gc/src/smaf/logger"
)

func init() {
	smafContext.NfInstanceID = uuid.New().String()
	SMAF_Self().PcfServiceUris = make(map[models.ServiceName]string)
	SMAF_Self().PcfSuppFeats = make(map[models.ServiceName]openapi.SupportedFeature)
	SMAF_Self().AMFStatusSubsData = make(map[string]AMFStatusSubscriptionData)
}

var smafContext SMAFContext

type SMAFContext struct {
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
	smafContext.UEAddressLock.Lock()
	defer smafContext.UEAddressLock.Unlock()
	smafContext.UEAddressTemp[3]++
	return smafContext.UEAddressTemp
}

func AllocateLocalSEID() uint64 {
	atomic.AddUint64(&smafContext.LocalSEIDCount, 1)
	return smafContext.LocalSEIDCount
}

func InitSmafContext(config *factory.Config) {
	if config == nil {
		logger.SMAFContextLog.Error("Config is nil")
		return
	}
	//20210610 added
	snRegex, err := regexp.Compile("5G:mnc[0-9]{3}[.]mcc[0-9]{3}[.]3gppnetwork[.]org")
	if err != nil {
		logger.SMAFContextLog.Warnf("SN compile error: %+v", err)
	} else {
		smafContext.snRegex = snRegex
	}
	//
	logger.SMAFContextLog.Infof("SmafConfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)
	configuration := config.Configuration
	if configuration.SmafName != "" {
		smafContext.Name = configuration.SmafName
	}

	sbi := configuration.Sbi
	if sbi == nil {
		logger.SMAFContextLog.Errorln("Configuration needs \"sbi\" value")
		return
	} else {
		smafContext.UriScheme = models.UriScheme(sbi.Scheme)
		smafContext.RegisterIPv4 = "127.0.0.1" // default localhost
		smafContext.SBIPort = 29502            // default port
		if sbi.RegisterIPv4 != "" {
			smafContext.RegisterIPv4 = sbi.RegisterIPv4
		}
		if sbi.Port != 0 {
			smafContext.SBIPort = sbi.Port
		}

		if tls := sbi.TLS; tls != nil {
			smafContext.Key = tls.Key
			smafContext.PEM = tls.PEM
		}

		smafContext.BindingIPv4 = os.Getenv(sbi.BindingIPv4)
		if smafContext.BindingIPv4 != "" {
			logger.SMAFContextLog.Info("Parsing ServerIPv4 address from ENV Variable.")
		} else {
			smafContext.BindingIPv4 = sbi.BindingIPv4
			if smafContext.BindingIPv4 == "" {
				logger.SMAFContextLog.Warn("Error parsing ServerIPv4 address as string. Using the 0.0.0.0 address as default.")
				smafContext.BindingIPv4 = "0.0.0.0"
			}
		}
	}
	//20210608 added ausf context initialize
	//smafContext.NfId = smafContext.NfInstanceID
	smafContext.Url = string(smafContext.UriScheme) + "://" + smafContext.RegisterIPv4 + ":" + strconv.Itoa(smafContext.SBIPort)
	smafContext.PlmnList = append(smafContext.PlmnList, configuration.PlmnSupportList...)

	if configuration.NrfUri != "" {
		smafContext.NrfUri = configuration.NrfUri
	} else {
		logger.SMAFContextLog.Warn("NRF Uri is empty! Using localhost as NRF IPv4 address.")
		smafContext.NrfUri = fmt.Sprintf("%s://%s:%d", smafContext.UriScheme, "127.0.0.1", 29510)
	}

	if pfcp := configuration.PFCP; pfcp != nil {
		if pfcp.Port == 0 {
			pfcp.Port = pfcpUdp.PFCP_PORT
		}
		pfcpAddrEnv := os.Getenv(pfcp.Addr)
		if pfcpAddrEnv != "" {
			logger.SMAFContextLog.Info("Parsing PFCP IPv4 address from ENV variable found.")
			pfcp.Addr = pfcpAddrEnv
		}
		if pfcp.Addr == "" {
			logger.SMAFContextLog.Warn("Error parsing PFCP IPv4 address as string. Using the 0.0.0.0 address as default.")
			pfcp.Addr = "0.0.0.0"
		}
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", pfcp.Addr, pfcp.Port))
		if err != nil {
			logger.SMAFContextLog.Warnf("PFCP Parse Addr Fail: %v", err)
		}

		smafContext.CPNodeID.NodeIdType = 0
		smafContext.CPNodeID.NodeIdValue = addr.IP.To4()
	}

	_, ipNet, err := net.ParseCIDR(configuration.UESubnet)
	if err != nil {
		logger.InitLog.Errorln(err)
	}

	smafContext.DNNInfo = configuration.DNN
	smafContext.UESubNet = ipNet
	smafContext.UEAddressTemp = ipNet.IP

	// Set client and set url
	ManagementConfig := Nnrf_NFManagement.NewConfiguration()
	ManagementConfig.SetBasePath(SMAF_Self().NrfUri)
	smafContext.NFManagementClient = Nnrf_NFManagement.NewAPIClient(ManagementConfig)

	NFDiscovryConfig := Nnrf_NFDiscovery.NewConfiguration()
	NFDiscovryConfig.SetBasePath(SMAF_Self().NrfUri)
	smafContext.NFDiscoveryClient = Nnrf_NFDiscovery.NewAPIClient(NFDiscovryConfig)

	smafContext.ULCLSupport = configuration.ULCL

	smafContext.SnssaiInfos = configuration.SNssaiInfo

	smafContext.OnlySupportIPv4 = true

	smafContext.UserPlaneInformation = NewUserPlaneInformation(&configuration.UserPlaneInformation)

	SetupNFProfile(config)
}

func InitSMFUERouting(routingConfig *factory.RoutingConfig) {

	if !smafContext.ULCLSupport {
		return
	}

	if routingConfig == nil {
		logger.SMAFContextLog.Error("configuration needs the routing config")
		return
	}

	logger.SMAFContextLog.Infof("ue routing config Info: Version[%s] Description[%s]",
		routingConfig.Info.Version, routingConfig.Info.Description)

	UERoutingInfo := routingConfig.UERoutingInfo
	smafContext.UEPreConfigPathPool = make(map[string]*UEPreConfigPaths)

	for _, routingInfo := range UERoutingInfo {
		supi := routingInfo.SUPI
		uePreConfigPaths, err := NewUEPreConfigPaths(supi, routingInfo.PathList)
		if err != nil {
			logger.SMAFContextLog.Warnln(err)
			continue
		}

		smafContext.UEPreConfigPathPool[supi] = uePreConfigPaths
	}

}

//// Create new SMAF context
func SMAF_Self() *SMAFContext {
	return &smafContext
}

func GetUserPlaneInformation() *UserPlaneInformation {
	return smafContext.UserPlaneInformation
}

func NewAusfUeContext(identifier string) (ausfUeContext *AusfUeContext) {
	ausfUeContext = new(AusfUeContext)
	ausfUeContext.Supi = identifier // supi
	return ausfUeContext
}

func AddAusfUeContextToPool(ausfUeContext *AusfUeContext) {
	smafContext.UePool.Store(ausfUeContext.Supi, ausfUeContext)
}

func CheckIfAusfUeContextExists(ref string) bool {
	_, ok := smafContext.UePool.Load(ref)
	return ok
}

func GetAusfUeContext(ref string) *AusfUeContext {
	context, _ := smafContext.UePool.Load(ref)
	ausfUeContext := context.(*AusfUeContext)
	return ausfUeContext
}

func AddSuciSupiPairToMap(supiOrSuci string, supi string) {
	newPair := new(SuciSupiMap)
	newPair.SupiOrSuci = supiOrSuci
	newPair.Supi = supi
	smafContext.suciSupiMap.Store(supiOrSuci, newPair)
}

func CheckIfSuciSupiPairExists(ref string) bool {
	_, ok := smafContext.suciSupiMap.Load(ref)
	return ok
}

func GetSupiFromSuciSupiMap(ref string) (supi string) {
	val, _ := smafContext.suciSupiMap.Load(ref)
	suciSupiMap := val.(*SuciSupiMap)
	supi = suciSupiMap.Supi
	return supi
}

func IsServingNetworkAuthorized(lookup string) bool {
	fmt.Println("in smaf IsServingNetworkAuthorized")
	if smafContext.snRegex.MatchString(lookup) {
		return true
	} else {
		return false
	}
}

/* replaced by SMAF_Self()
func AUSF_Self() *AUSFContext {
	return &smafContext
}
*/
/* replaced by SMAF_SelfID()
func (a *AUSFContext) AUSF_SelfID() string {
	return a.NfId
}
*/
func (a *SMAFContext) SMAF_SelfID() string {
	return a.NfInstanceID
}

// 20210618 added from pcf_context.go
func GetUri(name models.ServiceName) string {
	return smafContext.PcfServiceUris[name]
}
func (context *SMAFContext) GetIPv4Uri() string {
	return fmt.Sprintf("%s://%s:%d", context.UriScheme, context.RegisterIPv4, context.SBIPort)
}

//20210619 added from pcf_context.go
//SetDefaultUdrURI ... function to set DefaultUdrURI
func (context *SMAFContext) SetDefaultUdrURI(uri string) {
	context.DefaultUdrURILock.Lock()
	defer context.DefaultUdrURILock.Unlock()
	context.DefaultUdrURI = uri
}

// Find PcfUe which the policyId belongs to
func (context *SMAFContext) PCFUeFindByPolicyId(PolicyId string) *PCFUeContext {
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
func (context *SMAFContext) NewPCFUe(Supi string) (*PCFUeContext, error) {
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
func (context *SMAFContext) SessionBinding(req *models.AppSessionContextReqData) (*UeSmPolicyData, error) {
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
