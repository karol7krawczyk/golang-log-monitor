# Log Monitoring Application

![License](https://img.shields.io/badge/license-MIT-blue.svg)


## Table of Contents
- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration-file)
- [License](#license)

## Overview
This application is designed to monitor log files and broadcast updates over WebSocket to connected clients. It includes a web server to serve static files and handle WebSocket connections, a file system watcher to monitor changes in log files, and basic authentication for security.

## Features
- Accessing the Web Interface: Open a web browser and navigate to http://localhost:8080 (or the host and port specified in monitor_config.ini).
- WebSocket Connection: Connect to the WebSocket endpoint at ws://localhost:8080/ws to receive log updates.
- Basic Authentication: Use the credentials specified in the monitor_config.ini file to access the web interface.

## Installation
To get started, clone the repository and install the required dependencies.


### Clone the repository
```bash
git clone https://github.com/Karol7Krawczyk/golang-log-monitor.git
cd golang-log-monitor
```

### Run in docker
```bash
make build
make up
```

### The binary file is available after building the docker
```bash
./monitor
```

## Usage
The application monitors the specified log directories for changes. It broadcasts any new log entries over WebSocket to all connected clients.

- Accessing the Web Interface: Open a web browser and navigate to http://localhost:8080 (or the host and port specified in monitor_config.ini).
- WebSocket Connection: Connect to the WebSocket endpoint at ws://localhost:8080/ws to receive log updates.
- Basic Authentication: Use the credentials specified in the monitor_config.ini file to access the web interface.


## Configuration
The application uses a configuration file named config.ini to manage various settings, including authentication credentials and server details.

```bash
    [server]
    host = 0.0.0.0
    port = 8080

    [auth]
    username = admin
    password = admin

    [logs]
    directories = ./logs,./backup
```


## License
This project is licensed under the MIT License - see the LICENSE file for details.
