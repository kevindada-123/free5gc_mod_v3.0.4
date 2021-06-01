package context

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"sync"

	"free5gc/lib/openapi/models"
	"free5gc/src/ausf/factory"
	"free5gc/src/ausf/logger"

	"github.com/google/uuid"
)

type AUSFContext struct {
	suciSupiMap  sync.Map
	UePool       sync.Map
	NfId         string
	GroupID      string
	SBIPort      int
	RegisterIPv4 string
	BindingIPv4  string
	Url          string
	UriScheme    models.UriScheme
	NrfUri       string
	NfService    map[models.ServiceName]models.NfService
	PlmnList     []models.PlmnId
	UdmUeauUrl   string
	snRegex      *regexp.Regexp
}

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

var ausfContext AUSFContext

func Init() {
	InitAusfContext(&ausfContext)
}

//20210601 add
func InitAusfContext(context *AUSFContext) {
	snRegex, err := regexp.Compile("5G:mnc[0-9]{3}[.]mcc[0-9]{3}[.]3gppnetwork[.]org")
	if err != nil {
		logger.ContextLog.Warnf("SN compile error: %+v", err)
	} else {
		ausfContext.snRegex = snRegex
	}
	config := factory.AusfConfig
	logger.InitLog.Infof("ausfconfig Info: Version[%s] Description[%s]\n", config.Info.Version, config.Info.Description)

	configuration := config.Configuration
	sbi := configuration.Sbi

	ausfContext.NfId = uuid.New().String()
	ausfContext.GroupID = configuration.GroupId
	ausfContext.NrfUri = configuration.NrfUri
	ausfContext.UriScheme = models.UriScheme(configuration.Sbi.Scheme) // default uri scheme
	ausfContext.RegisterIPv4 = "127.0.0.1"                             // default localhost
	ausfContext.SBIPort = 29509                                        // default port
	if sbi != nil {
		if sbi.RegisterIPv4 != "" {
			ausfContext.RegisterIPv4 = sbi.RegisterIPv4
		}
		if sbi.Port != 0 {
			ausfContext.SBIPort = sbi.Port
		}

		if sbi.Scheme == "https" {
			ausfContext.UriScheme = models.UriScheme_HTTPS
		} else {
			ausfContext.UriScheme = models.UriScheme_HTTP
		}

		ausfContext.BindingIPv4 = os.Getenv(sbi.BindingIPv4)
		if context.BindingIPv4 != "" {
			logger.InitLog.Info("Parsing ServerIPv4 address from ENV Variable.")
		} else {
			ausfContext.BindingIPv4 = sbi.BindingIPv4
			if context.BindingIPv4 == "" {
				logger.InitLog.Warn("Error parsing ServerIPv4 address as string. Using the 0.0.0.0 address as default.")
				ausfContext.BindingIPv4 = "0.0.0.0"
			}
		}
	}

	ausfContext.Url = string(ausfContext.UriScheme) + "://" + ausfContext.RegisterIPv4 + ":" + strconv.Itoa(ausfContext.SBIPort)
	ausfContext.PlmnList = append(ausfContext.PlmnList, configuration.PlmnSupportList...)

	// context.NfService
	ausfContext.NfService = make(map[models.ServiceName]models.NfService)
	AddNfServices(&ausfContext.NfService, &config, &ausfContext)
	fmt.Println("ausf context = ", &ausfContext)
}

//20210601 add
func AddNfServices(serviceMap *map[models.ServiceName]models.NfService, config *factory.Config, context *AUSFContext) {
	var nfService models.NfService
	var ipEndPoints []models.IpEndPoint
	var nfServiceVersions []models.NfServiceVersion
	services := *serviceMap

	// nausf-auth
	nfService.ServiceInstanceId = context.NfId
	nfService.ServiceName = models.ServiceName_NAUSF_AUTH

	var ipEndPoint models.IpEndPoint
	ipEndPoint.Ipv4Address = context.RegisterIPv4
	ipEndPoint.Port = int32(context.SBIPort)
	ipEndPoints = append(ipEndPoints, ipEndPoint)

	var nfServiceVersion models.NfServiceVersion
	nfServiceVersion.ApiFullVersion = config.Info.Version
	nfServiceVersion.ApiVersionInUri = "v1"
	nfServiceVersions = append(nfServiceVersions, nfServiceVersion)

	nfService.Scheme = context.UriScheme
	nfService.NfServiceStatus = models.NfServiceStatus_REGISTERED

	nfService.IpEndPoints = &ipEndPoints
	nfService.Versions = &nfServiceVersions
	services[models.ServiceName_NAUSF_AUTH] = nfService
}
func NewAusfUeContext(identifier string) (ausfUeContext *AusfUeContext) {
	ausfUeContext = new(AusfUeContext)
	ausfUeContext.Supi = identifier // supi
	return ausfUeContext
}

func AddAusfUeContextToPool(ausfUeContext *AusfUeContext) {
	ausfContext.UePool.Store(ausfUeContext.Supi, ausfUeContext)
}

func CheckIfAusfUeContextExists(ref string) bool {
	_, ok := ausfContext.UePool.Load(ref)
	return ok
}

func GetAusfUeContext(ref string) *AusfUeContext {
	context, _ := ausfContext.UePool.Load(ref)
	ausfUeContext := context.(*AusfUeContext)
	return ausfUeContext
}

func AddSuciSupiPairToMap(supiOrSuci string, supi string) {
	newPair := new(SuciSupiMap)
	newPair.SupiOrSuci = supiOrSuci
	newPair.Supi = supi
	ausfContext.suciSupiMap.Store(supiOrSuci, newPair)
}

func CheckIfSuciSupiPairExists(ref string) bool {
	_, ok := ausfContext.suciSupiMap.Load(ref)
	return ok
}

func GetSupiFromSuciSupiMap(ref string) (supi string) {
	val, _ := ausfContext.suciSupiMap.Load(ref)
	suciSupiMap := val.(*SuciSupiMap)
	supi = suciSupiMap.Supi
	return supi
}

func IsServingNetworkAuthorized(lookup string) bool {
	if ausfContext.snRegex.MatchString(lookup) {
		return true
	} else {
		return false
	}
}

func AUSF_Self() *AUSFContext {
	return &ausfContext
}

func (a *AUSFContext) AUSF_SelfID() string {
	return a.NfId
}
