# Introduction

This program is waiting for an id button on a `MQTT` topic and execute the corresponding `LIRC` command.

# Configuration

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

# Unix socket to communicate with the LIRC daemon
lirc_socket: /var/run/lirc/lircd

# Device for LIRC communication
lirc_device: aircooling
```
