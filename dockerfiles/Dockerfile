# Description: Build a lobe image using pre-built executable
# copied from ./dist directory.
FROM golang:1.15-buster

# Set working directory to home
WORKDIR /home

# Get Lobe version from build command
ARG version
ENV version $version

COPY dist/lobe_${version}_Linux_x86_64.tar.gz .

# Download Lobe binary and its dependencies
RUN apt-get update && apt-get install git
RUN tar -xvf lobe_${version}_Linux_x86_64.tar.gz
RUN mv ./lob /usr/bin/lob

# Initialize lobe for testnet v1
RUN lob init -v \
    47shQ9ihsZBf2nYL6tAYR8q8Twb47KTNjimowxaNFRyGPL93oZL,\
    48LZFEsZsRPda1q2kiNZKToiTaeSx63GJdq6DWq9m9C4mSvWhHD,\
    48pFW5Yd5BLm4EVUJW8g9oG1BkNQz4wp2saLB8XmkvMRwRAB2FH,\
    48GKXaSLgJ5ox2C1jDshFGtD6Y4Zhd1doxK6iTDp3KCSZjzdWKt -k ${validator-key} -t 1595700581 --net=2000
