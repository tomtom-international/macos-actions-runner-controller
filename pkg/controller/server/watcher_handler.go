package server

import (
	"github.com/gorilla/websocket"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"net/http"
)

// TODO: validate origin
// TODO: remove debug logs
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func WatcherHandler(controller ControllerInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Errorf("Websocket connection upgrade failed: %v", err)
			return
		}
		defer conn.Close()
		logger.Debugf("Created new websocket connection and channel")
		nodesUpdateChan := make(chan types.WatcherNodesUpdate, 10)
		runnerUpdateChan := make(chan types.WatcherRunnersUpdate, 10)

		controller.AddNodesWatcher(conn, nodesUpdateChan)
		controller.AddRunnersWatcher(conn, runnerUpdateChan)

		for {
			select {
			case message, ok := <-nodesUpdateChan:
				if !ok {
					logger.Debugf("Watcher Handler channel closed")
					break
				}
				err = conn.WriteJSON(message)
				if err != nil {
					logger.Warnf("write: %v", err)
					controller.RemoveNodesWatcherHandler(conn)
					break
				}
			case message, ok := <-runnerUpdateChan:
				if !ok {
					logger.Debugf("Watcher Handler channel closed")
					break
				}
				err = conn.WriteJSON(message)
				if err != nil {
					logger.Warnf("write: %v", err)
					controller.RemoveRunnersWatcherHandler(conn)
					break
				}
			}
		}
	}
}
