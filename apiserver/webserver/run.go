package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	helpers "github.com/dgkanatsios/azuregameserversscalingkubernetes/apiserver/helpers"
	dgsv1alpha1 "github.com/dgkanatsios/azuregameserversscalingkubernetes/pkg/apis/azuregaming/v1alpha1"
	shared "github.com/dgkanatsios/azuregameserversscalingkubernetes/shared"
	"github.com/gorilla/mux"
)

var listPodStateRunningRequiresAuth = false

// Run begins the WebServer
func Run(port int, listrunningauth bool) error {

	router := mux.NewRouter()

	router.HandleFunc("/create", createDGSHandler).Queries("code", "{code}").Methods("GET")
	router.HandleFunc("/createcollection", createDGSColHandler).Queries("code", "{code}").Methods("POST")
	router.HandleFunc("/delete", deleteDGSHandler).Queries("name", "{name}", "code", "{code}").Methods("GET")
	route := router.HandleFunc("/running", getPodStateRunningDGSHandler).Methods("GET")
	if listrunningauth {
		listPodStateRunningRequiresAuth = true
		route.Queries("code", "{code}")
	}
	router.HandleFunc("/setactiveplayers", setActivePlayersHandler).Methods("POST")
	router.HandleFunc("/setserverstatus", setServerStatusHandler).Methods("POST")

	//this should be the last handler
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./html/"))).Methods("GET")

	log.Printf("Waiting for requests at port %d\n", port)

	return http.ListenAndServe(fmt.Sprintf(":%s", strconv.Itoa(port)), router)
}

func createDGSHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("create was called")

	result, err := helpers.IsAPICallAuthenticated(w, r)
	if err != nil {
		log.Errorf("Error in authentication: %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Error"))
		return
	}

	if !result {
		w.WriteHeader(401)
		w.Write([]byte("Unathorized"))
		return
	}

	var dgsInfo helpers.DedicatedGameServerInfo
	err = json.NewDecoder(r.Body).Decode(&dgsInfo)

	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte("Incorrect arguments: " + err.Error()))
		return
	}

	podname, err := helpers.CreateDedicatedGameServerCRD(dgsInfo)

	if err != nil {
		log.Printf("error encountered: %s", err.Error())
		w.Write([]byte(fmt.Sprintf("Error %s encountered", err.Error())))
	} else {
		w.Write([]byte("DedicatedGameServer " + podname + " was created"))
	}
}

func createDGSColHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("createcollection was called")

	result, err := helpers.IsAPICallAuthenticated(w, r)
	if err != nil {
		log.Errorf("Error in authentication: %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Error"))
		return
	}

	if !result {
		w.WriteHeader(401)
		w.Write([]byte("Unathorized"))
		return
	}

	var dgsColInfo helpers.DedicatedGameServerCollectionInfo
	err = json.NewDecoder(r.Body).Decode(&dgsColInfo)

	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte("Incorrect arguments: " + err.Error()))
		return
	}

	colname, err := helpers.CreateDedicatedGameServerCollectionCRD(dgsColInfo)

	if err != nil {
		log.Printf("error encountered: %s", err.Error())
		w.Write([]byte(fmt.Sprintf("Error %s encountered", err.Error())))
	} else {
		w.Write([]byte("DedicatedGameServerCollection " + colname + " was created"))
	}
}

func deleteDGSHandler(w http.ResponseWriter, r *http.Request) {

	result, err := helpers.IsAPICallAuthenticated(w, r)
	if err != nil {
		log.Errorf("Error in authentication: %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Error"))
		return
	}

	if !result {
		w.WriteHeader(401)
		w.Write([]byte("Unathorized"))
		return
	}

	name := r.FormValue("name")

	_, dgsClient, err := shared.GetClientSet()

	if err != nil {
		log.Errorf("Error in getting client set: %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Error"))
		return
	}

	err = dgsClient.AzuregamingV1alpha1().DedicatedGameServers(shared.GameNamespace).Delete(name, nil)
	if err != nil {
		msg := fmt.Sprintf("Cannot delete DedicatedGameServer due to %s", err.Error())
		log.Print(msg)
		w.Write([]byte(msg))
		return
	}

	w.Write([]byte(name + " was deleted"))
}

func getPodStateRunningDGSHandler(w http.ResponseWriter, r *http.Request) {

	if listPodStateRunningRequiresAuth {
		result, err := helpers.IsAPICallAuthenticated(w, r)
		if err != nil {
			log.Errorf("Error in authentication: %v", err)
			w.WriteHeader(500)
			w.Write([]byte("Error"))
			return
		}

		if !result {
			w.WriteHeader(401)
			w.Write([]byte("Unathorized"))
			return
		}
	}

	entities, err := shared.GetDedicatedGameServersPodStateRunning()
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Error in marshaling to JSON: " + err.Error()))
	}
	result, err := json.Marshal(entities)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Error in marshaling to JSON: " + err.Error()))
	}
	w.Write(result)
}

func setActivePlayersHandler(w http.ResponseWriter, r *http.Request) {
	result, err := helpers.IsAPICallAuthenticated(w, r)
	if err != nil {
		log.Errorf("Error in authentication: %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Error"))
		return
	}

	if !result {
		w.WriteHeader(401)
		w.Write([]byte("Unathorized"))
		return
	}

	var serverActivePlayers helpers.ServerActivePlayers
	err = json.NewDecoder(r.Body).Decode(&serverActivePlayers)

	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Error setting Active Players: " + err.Error()))
		return
	}

	err = shared.UpdateActivePlayers(serverActivePlayers.ServerName, serverActivePlayers.PlayerCount)

	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Error setting Active Players: " + err.Error()))
		return
	}

	w.Write([]byte(fmt.Sprintf("Set active players OK for server: %s\n", serverActivePlayers.ServerName)))
}

func setServerStatusHandler(w http.ResponseWriter, r *http.Request) {
	result, err := helpers.IsAPICallAuthenticated(w, r)
	if err != nil {
		log.Errorf("Error in authentication: %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Error"))
		return
	}

	if !result {
		w.WriteHeader(401)
		w.Write([]byte("Unathorized"))
		return
	}

	var serverStatus helpers.ServerStatus
	err = json.NewDecoder(r.Body).Decode(&serverStatus)

	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Error setting ServerStatus: " + err.Error()))
		return
	}

	status := dgsv1alpha1.DedicatedGameServerState(serverStatus.Status)
	//a very simple validation
	if status != dgsv1alpha1.DedicatedGameServerStateCreating && status != dgsv1alpha1.DedicatedGameServerStateMarkedForDeletion && status != dgsv1alpha1.DedicatedGameServerStateRunning && status != dgsv1alpha1.DedicatedGameServerStateFailed {
		w.WriteHeader(400)
		w.Write([]byte("Wrong value for serverStatus"))
		return
	}

	err = shared.UpdateGameServerStatus(serverStatus.ServerName, status)

	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Error setting ServerStatus: " + err.Error()))
		return
	}

	w.Write([]byte(fmt.Sprintf("Set server status OK for server: %s\n", serverStatus.ServerName)))
}
