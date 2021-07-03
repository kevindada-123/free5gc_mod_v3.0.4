package context

import (
	"fmt"
	"free5gc/lib/idgenerator"
	"free5gc/src/smpcf/factory"
	"free5gc/src/smpcf/logger"
	"math"
)

type UEPreConfigPaths struct {
	DataPathPool    DataPathPool
	PathIDGenerator *idgenerator.IDGenerator
}

func NewUEDataPathNode(name string) (node *DataPathNode, err error) {

	upNodes := smpcfContext.UserPlaneInformation.UPNodes

	if _, exist := upNodes[name]; !exist {
		err = fmt.Errorf("UPNode %s isn't exist in smfcfg.conf, but in UERouting.yaml!", name)
		return nil, err
	}

	node = &DataPathNode{
		UPF:            upNodes[name].UPF,
		UpLinkTunnel:   &GTPTunnel{},
		DownLinkTunnel: &GTPTunnel{},
	}
	return
}

func NewUEPreConfigPaths(SUPI string, paths []factory.Path) (*UEPreConfigPaths, error) {
	var uePreConfigPaths *UEPreConfigPaths
	ueDataPathPool := NewDataPathPool()
	lowerBound := 0
	pathIDGenerator := idgenerator.NewGenerator(1, math.MaxInt32)

	logger.PduSessLog.Infoln("In NewUEPreConfigPaths")

	for idx, path := range paths {
		upperBound := len(path.UPF) - 1
		dataPath := NewDataPath()

		if idx == 0 {
			dataPath.IsDefaultPath = true
		}

		var pathID int64
		if allocPathID, err := pathIDGenerator.Allocate(); err != nil {
			logger.SMPCFContextLog.Warnf("Allocate pathID error: %+v", err)
			return nil, err
		} else {
			pathID = allocPathID
		}

		dataPath.Destination.DestinationIP = path.DestinationIP
		dataPath.Destination.DestinationPort = path.DestinationPort
		ueDataPathPool[pathID] = dataPath
		var ueNode, childNode, parentNode *DataPathNode
		for idx, nodeName := range path.UPF {

			if newUeNode, err := NewUEDataPathNode(nodeName); err != nil {
				return nil, err
			} else {
				ueNode = newUeNode
			}

			switch idx {
			case lowerBound:
				childName := path.UPF[idx+1]
				if newChildNode, err := NewUEDataPathNode(childName); err != nil {
					logger.SMPCFContextLog.Warnln(err)
				} else {
					childNode = newChildNode
					ueNode.AddNext(childNode)
					dataPath.FirstDPNode = ueNode
				}

			case upperBound:
				childNode.AddPrev(parentNode)
			default:
				childNode.AddPrev(parentNode)
				ueNode = childNode
				childName := path.UPF[idx+1]
				if childNode, err := NewUEDataPathNode(childName); err != nil {
					logger.SMPCFContextLog.Warnln(err)
				} else {
					ueNode.AddNext(childNode)
				}

			}

			parentNode = ueNode

		}

		logger.SMPCFContextLog.Traceln("New data path added")
		logger.SMPCFContextLog.Traceln("\n" + dataPath.ToString() + "\n")
	}

	uePreConfigPaths = &UEPreConfigPaths{
		DataPathPool:    ueDataPathPool,
		PathIDGenerator: pathIDGenerator,
	}
	return uePreConfigPaths, nil
}

func GetUEPreConfigPaths(SUPI string) *UEPreConfigPaths {
	return smpcfContext.UEPreConfigPathPool[SUPI]
}

func CheckUEHasPreConfig(SUPI string) (exist bool) {
	_, exist = smpcfContext.UEPreConfigPathPool[SUPI]
	fmt.Println("CheckUEHasPreConfig")
	fmt.Println(smpcfContext.UEPreConfigPathPool)
	return
}