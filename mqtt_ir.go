package main

import (
    "fmt"
    mqtt "github.com/eclipse/paho.mqtt.golang"
    "github.com/ilyakaznacheev/cleanenv"
    "github.com/chbmuc/lirc"
    "errors"
    "time"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "encoding/binary"
    "unsafe"
)

type ConfigDatabase struct {
    // MQTT Configuration
    ServerAddress       string `yaml:"server_adress" env:"MQTT_SERVER_ADDRESS" env-default:"tcp://localhost:1883"`
    User                string  `yaml:"user" env:"MQTT_USER" env-default:"user"`
    Password            string  `yaml:"password" env:"MQTT_PASSWORD" env-default:"password"`
    ClientID            string  `yaml:"clientid" env:"MQTT_CLIENTID" env-default:"ir_client"`
    Topic               string  `yaml:"topic" env:"MQTT_TOPIC" env-default:"topic"`

    // Mode : High level or low level
    Mode                string `yaml:"mode" env:"MQTT_IR_MODE" env-default:"low"`

    // High level Lirc Configuration
    LircSocket          string  `yaml:"lirc_socket" env:"LIRC_SOCKET" env-default:"/var/run/lirc/lircd"`
    LircDevice          string  `yaml:"lirc_device" env:"LIRC_DEVICE" env-default:"lircdevice"`

    // Low Level IR driver configuration
    IRDriver            string `yaml:"ir_driver" env:"IR_DRIVER" env-default:"/dev/lirc0"`
    IRFreq              int    `yaml:"ir_freq" env:"IR_FREQ" env-default:"38000"`
    IRDutyCycle         int    `yaml:"ir_dutycycle" env:"IR_DUTYCYCLE" env-default:"50"`
}

// IOCTL values when communicating directly with the driver
const (
    LIRC_SET_SEND_CARRIER uint32    = 0x40046913
    LIRC_SET_SEND_DUTY_CYCLE uint32 = 0x40046915
)

var cfg ConfigDatabase
var IR *lirc.Router


func handle_incoming_mqtt_message(client mqtt.Client, msg mqtt.Message) {
    var err error
    fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
    if cfg.Mode == "low" {
        err = send_lowlevel_ir(msg.Payload(),&cfg)
    } else {
        err = IR.Send(fmt.Sprintf("%s %s",cfg.LircDevice,msg.Payload()))
    }
    if err != nil {
        fmt.Printf("Error sending IR data: %+v\n",err)
    }
}

// Change a hexvalue string into a string representing
// sequence of pulses/spaces understandable by the driver
func from_hexvalue_to_buffer(value string) ([]byte,error) {
    var pulses_spaces []uint32
    var buffer []byte

    // A
    pulses_spaces = append(pulses_spaces,4350)
    pulses_spaces = append(pulses_spaces,4350)

    // Transform the HEX String value to a list of pulses and spaces
    for i := 0; i < len(value); i += 2 {
        x,err := strconv.ParseUint(value[i:i+2],16,8)
        if err != nil {
            fmt.Printf("Unable to decode hex string %s",value[i:i+1])
            return nil,errors.New("unable hex string decode")
        }

        // Extract each bit from the string
        // If 0 then (460,460), otherwise (460,1600)
        for j := 0; j < 8; j++ {
            pulses_spaces = append(pulses_spaces,460)
            // if bit j is different from 0
            if ((1<<uint(7-j)) & x) != 0 {
                pulses_spaces = append(pulses_spaces,1600)
            } else {
                pulses_spaces = append(pulses_spaces,600)
            }
        }
    }

    // Transform the list of pulses/spaces in a byte array
    for i:=0; i<len(pulses_spaces); i+=2 {
        bs := make([]byte, 8)
        binary.LittleEndian.PutUint32(bs[:4], pulses_spaces[i])
        binary.LittleEndian.PutUint32(bs[4:], pulses_spaces[i+1])
        buffer = append(buffer,bs...)
    }
    buffer = append(buffer,0xCA,0x01,0x00,0x00)

    return buffer,nil
}

// Send the hexvalue string as a IR signal
func send_lowlevel_ir(value []byte, conf *ConfigDatabase) error {
    // Initialize driver
    fd, err := syscall.Open(conf.IRDriver, syscall.O_RDWR, 0666)
    if err != nil {
        return errors.New(fmt.Sprintf("Unable to open %s : %+v\n", conf.IRDriver, err))
    }

    _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(LIRC_SET_SEND_CARRIER), uintptr(unsafe.Pointer(&cfg.IRFreq)))
    if errno != 0 {
        return errors.New(fmt.Sprintf("Unable to ioctl LIRC_SET_SEND_CARRIER : %+v", err))
    }
    _, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(LIRC_SET_SEND_DUTY_CYCLE), uintptr(unsafe.Pointer(&cfg.IRDutyCycle)))
    if errno != 0 {
        return errors.New(fmt.Sprintf("Unable to ioctl LIRC_SET_SEND_DUTY_CYCLE : %+v", errno))
    }

    // Build buffer bytes
    buf,err := from_hexvalue_to_buffer(string(value))
    if err != nil {
        return errors.New(fmt.Sprintf("Unable to unpack hex value string"))
    }

    _,err = syscall.Write(fd, buf)
    if err != nil {
        return errors.New(fmt.Sprintf("Unable to send to driver : %+v",err))
    }

    _ = syscall.Close(fd)

    return nil
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
    if cfg.Mode != "low" {
        create_ir_client(&cfg)
    }

    // Messages will be delivered asynchronously so we just need to wait for a signal to shutdown
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, os.Interrupt)
    signal.Notify(sig, syscall.SIGTERM)

    <-sig
    fmt.Println("signal caught - exiting")
    client.Disconnect(1000)
}

