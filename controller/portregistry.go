package controller

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"github.com/dgkanatsios/azuregameserversscalingkubernetes/shared"

	dgsclientset "github.com/dgkanatsios/azuregameserversscalingkubernetes/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var portRegistry *IndexedDictionary
var clientset *dgsclientset.Clientset
var mutex = &sync.Mutex{}

func InitializePortRegistry(dgsclientset *dgsclientset.Clientset) error {
	clientset = dgsclientset

	portRegistry = NewIndexedDictionary(shared.MinPort, shared.MaxPort)

	dgsList, err := clientset.AzuregamingV1alpha1().DedicatedGameServers(shared.GameNamespace).List(metav1.ListOptions{})
	if err != nil {
		log.Error("Error getting Dedicated Game Servers List: %v", err)
		return err
	}

	if len(dgsList.Items) > 0 {
		for _, dgs := range dgsList.Items {
			ports := make([]int32, len(dgs.Spec.Ports))
			for i, portInfo := range dgs.Spec.Ports {
				ports[i] = portInfo.HostPort
			}
			portRegistry.AssignRegisteredPorts(ports, dgs.Name)
		}
	}

	portRegistry.AssignUnregisteredPorts()

	return nil

}

func (id *IndexedDictionary) displayRegistry() {
	fmt.Printf("-------------------------------------\n")
	fmt.Printf("Ports: %v\n", id.Ports)
	fmt.Printf("GameServerPorts: %s\n", id.GameServerPorts)
	fmt.Printf("Indexes: %v\n", id.Indexes)
	fmt.Printf("NextIndex: %d\n", id.NextFreePortIndex)
	fmt.Printf("-------------------------------------\n")
}

func (id *IndexedDictionary) GetNewPort(serverName string) (int32, error) {

	if id == nil {
		log.Panic("PortRegistry is not initialized")
	}

	mutex.Lock()
	defer mutex.Unlock()

	initialIndex := id.NextFreePortIndex
	for {
		if id.Ports[id.Indexes[id.NextFreePortIndex]] == false {
			//we found a port
			port := id.Indexes[id.NextFreePortIndex]
			id.Ports[port] = true
			id.GameServerPorts[serverName] = fmt.Sprintf("%d,%s", port, id.GameServerPorts[serverName])
			id.IncreaseNextFreePortIndex()
			return port, nil
		}

		id.IncreaseNextFreePortIndex()

		if initialIndex == id.NextFreePortIndex {
			//we did a full loop - no empty ports
			return 0, errors.New("Cannot register a new port")
		}
	}

}

func (id *IndexedDictionary) DeregisterServerPorts(serverName string) {

	mutex.Lock()
	defer mutex.Unlock()

	ports := strings.Split(id.GameServerPorts[serverName], ",")

	var deleteErrors string

	for _, port := range ports {
		if port != "" {
			portInt, errconvert := strconv.Atoi(port)
			if errconvert != nil {
				deleteErrors = fmt.Sprintf("%s,%s", deleteErrors, errconvert.Error())
			}

			id.Ports[int32(portInt)] = false
		}
	}

	delete(id.GameServerPorts, serverName)

}

type IndexedDictionary struct {
	Ports             map[int32]bool
	GameServerPorts   map[string]string
	Indexes           []int32
	NextFreePortIndex int32
	Min               int32
	Max               int32
}

func NewIndexedDictionary(min, max int32) *IndexedDictionary {
	id := &IndexedDictionary{
		Ports:           make(map[int32]bool, max-min+1),
		GameServerPorts: make(map[string]string, max-min+1),
		Indexes:         make([]int32, max-min+1),
		Min:             min,
		Max:             max,
	}
	return id
}

func (id *IndexedDictionary) AssignRegisteredPorts(ports []int32, serverName string) {
	mutex.Lock()
	defer mutex.Unlock()

	var portsString string
	for i := 0; i < len(ports); i++ {
		id.Ports[ports[i]] = true
		id.Indexes[i] = ports[i]
		id.IncreaseNextFreePortIndex()

		portsString = fmt.Sprintf("%d,%s", ports[i], portsString)

	}
	id.GameServerPorts[serverName] = portsString

}

func (id *IndexedDictionary) AssignUnregisteredPorts() {
	mutex.Lock()
	defer mutex.Unlock()

	i := id.NextFreePortIndex
	for _, port := range id.getPermutatedPorts() {
		if _, ok := id.Ports[port]; !ok {
			id.Ports[port] = false
			id.Indexes[i] = port
			i++
		}
	}
}

func (id *IndexedDictionary) IncreaseNextFreePortIndex() {
	id.NextFreePortIndex++
	//reset the index if needed
	if id.NextFreePortIndex == id.Max-id.Min+1 {
		id.NextFreePortIndex = 0
	}

}

func (id *IndexedDictionary) getPermutatedPorts() []int32 {
	ports := make([]int32, id.Max-id.Min+1)
	for i := 0; i < len(ports); i++ {
		ports[i] = int32(id.Min)
	}
	perm := rand.Perm(int((id.Max - id.Min + 1)))

	for i := 0; i < len(ports); i++ {
		ports[i] += int32(perm[i])
	}
	return ports
}
