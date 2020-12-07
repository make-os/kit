# Description: Build a kit image using pre-built executable fetched from external URL.
FROM golang:1.15-buster

# Expose ports
EXPOSE 9000
EXPOSE 9002
EXPOSE 9003
EXPOSE 9004

# Set working directory to home
WORKDIR /home

# Get Kit version from build command
ARG version
ENV version $version
ARG vKey
ENV vKey $vKey

# Download Kit binary and its dependencies
RUN apt-get update && apt-get install git
RUN curl https://storage.googleapis.com/kit-bin/kit_${version}_Linux_x86_64.tar.gz | tar -xz
RUN mv ./kit /usr/bin/kit

# Initialize kit for testnet v1 with 4 validators.
RUN kit init --net=2000 -v=47shQ9ihsZBf2nYL6tAYR8q8Twb47KTNjimowxaNFRyGPL93oZL,48LZFEsZsRPda1q2kiNZKToiTaeSx63GJdq6DWq9m9C4mSvWhHD,48pFW5Yd5BLm4EVUJW8g9oG1BkNQz4wp2saLB8XmkvMRwRAB2FH,48GKXaSLgJ5ox2C1jDshFGtD6Y4Zhd1doxK6iTDp3KCSZjzdWKt -k=${vKey} -t=1595700581

# Start the node (in dev mode)
ENTRYPOINT ["kit", "start", "--dev", "--rpc.on"]