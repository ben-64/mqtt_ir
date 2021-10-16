# Introduction

This program can be used in two different modes:
- Either it is waiting for an id button on a `MQTT` topic and execute the corresponding `LIRC` command, by communicating with the `lircd` unix socket
- Or it is waiting for an IR command, and send it directly to the IR driver, bypassing `lircd`

# Configuration to use `lircd`

This program can be configured through a configuration file called `config.yml` in the same folder.

```conf
# Connection string for the MQTT server
server_adress: tcp://mqtt_server:1883

# User for MQTT connection
user: mqtt_user

# Password for MQTT connection
password: mqtt_password

# ID Client in MQTT communication
clientid: ir_mqtt

# Topic to subscribe to
topic: ir

# High level mode, meaning that we are going to use lircd
mode: high

# Unix socket to communicate with the LIRC daemon
lirc_socket: /var/run/lirc/lircd

# Device for LIRC communication
lirc_device: aircooling
```

# Configuration for interfacing with the default IR driver

```conf
# Connection string for the MQTT server
server_adress: tcp://mqtt_server:1883

# User for MQTT connection
user: mqtt_user

# Password for MQTT connection
password: mqtt_password

# ID Client in MQTT communication
clientid: ir_mqtt

# Topic to subscribe to
topic: ir

# High level mode, meaning that we are going to use lircd
mode: low

# Frequency to give to the driver
ir_frequency: 38000

# Duty cycle for the driver
ir_dutycycle: 50
```
