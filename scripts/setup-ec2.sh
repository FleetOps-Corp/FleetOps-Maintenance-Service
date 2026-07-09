#!/usr/bin/env bash

sudo apt update && apt install -y docker.io docker-compose
sudo usermod - aG docker $USER && newgrp docker
