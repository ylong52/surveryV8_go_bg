module ws_server

go 1.24.2

require (
	github.com/gorilla/websocket v1.5.3
	survey.cli v0.0.0
)

require (
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace survey.cli => ..
