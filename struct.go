package main

// ConfigJson
type ConfigJson struct {
	Username, Password, ClientID, Broker, CAPath, Location string
}

// MqPayload
type MqPayload struct {
	Location, Device, Command string
}
