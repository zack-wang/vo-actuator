package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	//"os/exec"
	"os/signal"
	"path/filepath"
	"time"
)

var (
	WD, clientid, mylocation                       string
	broker, brokerUser, brokerPass, capath, prefix string
	ver                                            = flag.Bool("v", false, `show version`)
	rel                                            = flag.Bool(`l`, false, `show release note`)
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	//log.SetLevel(log.WarnLevel)
}

func main() {
	flag.Parse()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill)

	if *ver {
		log.WithFields(log.Fields{
			"version": *ver,
		}).Info("show version")
	}
	if *rel {
		log.WithFields(log.Fields{
			"release notes": *rel,
		}).Info("show notes")
	}

	WD, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Error(err.Error())
	}
	//WD = filepath.Dir(wd + `..`)

	//LOGDIR := filepath.Dir(WD + `/bin`)
	log.Info("Work Directory=", WD)
	jsonFile, err := os.Open(WD + `/config.json`)
	if err == nil {
		byteValue, err := ioutil.ReadAll(jsonFile)
		if err == nil {
			cfg := ConfigJson{}
			err = json.Unmarshal(byteValue, &cfg)
			if err == nil {
				broker = cfg.Broker
				brokerUser = cfg.Username
				brokerPass = cfg.Password
				capath = cfg.CAPath
				prefix = `actions/` + brokerUser + `/`
				clientid = cfg.ClientID
				mylocation = cfg.Location
			}
		}
	}

	// 處理雲端命令， callback function when recevie payload from broker
	var f3 mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		//mid,cmd,path:=getMidCmdPath(msg.Topic())
		pl := string(msg.Payload())
		top := msg.Topic()
		log.WithFields(log.Fields{
			"Topic":   top,
			"Payload": pl,
		}).Info("Receive from broker")

		act := MqPayload{}
		jerr := json.Unmarshal([]byte(pl), &act)
		if jerr == nil {
			log.WithFields(log.Fields{
				"Location": act.Location,
				"Device":   act.Device,
				"Command":  act.Command,
			}).Info("Payload decoded")

			// 板子的位置要與雲端發下來的位置相同才做事情， location must match otherwise drop this ACTION
			if act.Location == mylocation {
				switch act.Device {
				/*
					case `ls`, `GetList`:
						_, err := exec.Command(`/bin/bash`, `-c`, `./query.sh `+dbname+` "`+sql+`"`).Output()
						msg, err := exec.Command(`ls`, `-l`, `/digiwin/dot/`+path).Output()
						if err == nil {
							//if tkn := client.Publish(prefix+`up/ls/`+path, 1, false, []byte(msg)); tkn.Wait() && tkn.Error() != nil {
							if tkn := client.Publish(prefix+uid+`/SHOW/`+sid, 1, false, []byte(msg)); tkn.Wait() && tkn.Error() != nil {
								mlog.Error.Println(`Pub error`)
							}
						} else {
							mlog.Error.Println(`Exec error=`, err)
						}
				*/
				case `fan`:
					//GPIO(20,true)
				}
				res := MqPayload{Location: act.Location, Device: act.Device, Command: `Yeah, I got it`}
				byt, jerr := json.Marshal(res)
				if jerr == nil {
					if tkn := client.Publish(prefix+`res`, 1, false, byt); tkn.Wait() && tkn.Error() != nil {
						log.Error(tkn.Error())
					}
				} else {
					log.Error(`json marshal error`)
				}
			}
		} else {
			log.Error(`Unmarshal Error=`, jerr.Error())
		}
	}

	var f1 mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
		//mlog.Error.Println(`Conntion Lost:`, err)
	}
	var f2 mqtt.OnConnectHandler = func(client mqtt.Client) {
		//mlog.Info.Println(`Connected:`, broker)
		if tkn := client.Publish(prefix+`lwm`, 1, true, []byte(clientid+` online `+time.Now().Format(`2006-01-02T15:04:05.000`))); tkn.Wait() && tkn.Error() != nil {
			log.Error(`Pub error`)
		} else {
			log.Info(`Connected.`)
		}

		// SUB req
		if token := client.Subscribe(prefix+`req`, 2, f3); token.Wait() && token.Error() != nil { //Publish(topic string, qos byte, retained bool, payload interface{})
			// panic(token.Error())
			log.Error("MQTT sub error:")
		} else {
			log.Info(`Command topic=` + prefix + `req`)
		}
	}

	opts := mqtt.NewClientOptions().AddBroker(broker)
	if broker[0:3] == "ssl" { //判斷是否為加密連線
		roots := x509.NewCertPool()
		pemCerts, err := ioutil.ReadFile(capath)
		if err == nil {
			roots.AppendCertsFromPEM(pemCerts)
		}
		tlsconf := &tls.Config{RootCAs: roots, ClientAuth: tls.NoClientCert, ClientCAs: nil}
		opts.SetTLSConfig(tlsconf)
	}
	opts.SetClientID(clientid) // 自動clientID
	opts.SetCleanSession(true)
	opts.SetMaxReconnectInterval(1 * time.Second)
	opts.SetAutoReconnect(true) //自動重連
	opts.SetMessageChannelDepth(1000)
	opts.SetConnectionLostHandler(f1)     //連線丟失的動作
	opts.SetOnConnectHandler(f2)          //連線丟失重連後的執行動作，重訂閱Topic
	opts.SetPingTimeout(60 * time.Second) // 60 秒發ping，防止斷線
	opts.SetKeepAlive(300 * time.Second)
	if brokerUser != "" {
		opts.SetUsername(brokerUser)
		opts.SetPassword(brokerPass)
	}
	opts.SetWill(prefix+`lwm`, clientid+` offline, last connected at`+time.Now().Format(`2006-01-02T15:04:05.000`), 1, true)
	mqc := mqtt.NewClient(opts)
	for i := 0; i < 5; i++ {
		if token := mqc.Connect(); token.Wait() && token.Error() != nil {
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	defer func() {
		log.Info(`mqtt client closed`)
		mqc.Disconnect(1000)
	}()

	t := time.NewTicker(600 * time.Second)
	for {
		select {
		case <-t.C:
			log.Info(`.`)
		case <-interrupt:
			log.Info("Interrupted !")
			return
		}
	}

}
