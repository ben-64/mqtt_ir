package main

import (
    "fmt"
    mqtt "github.com/eclipse/paho.mqtt.golang"
    "github.com/ilyakaznacheev/cleanenv"
    "github.com/chbmuc/lirc"
    "time"
    "os"
    "os/signal"
    "syscall"
)

type ConfigDatabase struct {
    // MQTT Configuration
    ServerAddress     string `yaml:"server_adress" env:"MQTT_SERVER_ADDRESS" env-default:"tcp://localhost:1883"`
    User              string `yaml:"user" env:"MQTT_USER" env-default:"user"`
    Password          string `yaml:"password" env:"MQTT_PASSWORD" env-default:"password"`
    ClientID          string `yaml:"clientid" env:"MQTT_CLIENTID" env-default:"ir_client"`
    Topic             string `yaml:"topic" env:"MQTT_TOPIC" env-default:"topic"`

    // Lirc Configuration
    LircSocket          string `yaml:"lirc_socket" env:"LIRC_SOCKET" env-default:"/var/run/lirc/lircd"`
    LircDevice          string `yaml:"lirc_device" env:"LIRC_DEVICE" env-default:"lircdevice"`
}

var cfg ConfigDatabase
var IR *lirc.Router


func handle_incoming_mqtt_message(client mqtt.Client, msg mqtt.Message) {
    fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
    err := IR.Send(fmt.Sprintf("%s %s",cfg.LircDevice,msg.Payload()))
    if err != nil {
        fmt.Printf("Error sending LIRC data: %+v\n",err)
    }
}


// Create MQTT Client
func create_mqtt_client(conf *ConfigDatabase) mqtt.Client {
    var mqtt_qos byte = 1

    // Now we establish the connection to the mqtt broker
    opts := mqtt.NewClientOptions()
    opts.AddBroker(conf.ServerAddress)
    opts.SetClientID(conf.ClientID)
    opts.SetUsername(conf.User)
    opts.SetPassword(conf.Password)

    opts.ConnectTimeout = time.Second // Minimal delays on connect
    opts.WriteTimeout = time.Second   // Minimal delays on writes
    opts.KeepAlive = 10               // Keepalive every 10 seconds so we quickly detect network outages
    opts.PingTimeout = time.Second    // local broker so response should be quick

    // Automate connection management (will keep trying to connect and will reconnect if network drops)
    opts.ConnectRetry = true
    opts.AutoReconnect = true

    // Log events
    opts.OnConnect = func(client mqtt.Client) {
        fmt.Println("Connected")
    }
    opts.OnConnectionLost = func(cl mqtt.Client, err error) {
        fmt.Println("connection lost")
    }
    opts.OnReconnecting = func(mqtt.Client, *mqtt.ClientOptions) {
        fmt.Println("attempting to reconnect")
    }

    client := mqtt.NewClient(opts)
    client.AddRoute(conf.Topic,handle_incoming_mqtt_message)
    if token := client.Connect(); token.Wait() && token.Error() != nil {
        panic(token.Error())
    }

    token := client.Subscribe(conf.Topic, mqtt_qos, nil)
    token.Wait()

    return client
}

// Create IR Client
func create_ir_client(conf *ConfigDatabase) {
    // Initialize with path to lirc socket
    var err error
    IR, err = lirc.Init(conf.LircSocket)
    if err != nil {
        panic(err)
    }
}

func main() {
    err := cleanenv.ReadConfig("config.yml", &cfg)
    if err != nil {
        fmt.Printf("Unable to open %s configuration file\n","config.yml")
    }
    client := create_mqtt_client(&cfg)
    create_ir_client(&cfg)

    // Messages will be delivered asynchronously so we just need to wait for a signal to shutdown
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, os.Interrupt)
    signal.Notify(sig, syscall.SIGTERM)

    <-sig
    fmt.Println("signal caught - exiting")
    client.Disconnect(1000)
}

