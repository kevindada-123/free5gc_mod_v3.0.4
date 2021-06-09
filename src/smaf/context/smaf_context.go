package context

import (
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"

	//20210608 added ausf context
	"regexp"
	"strconv"

	"github.com/google/uuid"

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

	//20210608 added AUSFContext
	suciSupiMap sync.Map
	UePool      sync.Map
	//NfId        string
	GroupID    string
	Url        string
	NfService  map[models.ServiceName]models.NfService
	PlmnList   []models.PlmnId
	UdmUeauUrl string
	snRegex    *regexp.Regexp
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

func SMAF_Self() *SMAFContext {
	return &smafContext
}

func GetUserPlaneInformation() *UserPlaneInformation {
	return smafContext.UserPlaneInformation
}
