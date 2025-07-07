#!/bin/bash

find . -name "*.go" | xargs wc -l | tail -n 1 | awk '{print $1}'
