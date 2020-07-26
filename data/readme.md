# Data File Directory

This directory contains data files that will be read by other 
packages for processing. Files accessed by other packages will be embedded in the 
program executable using Packr.

### genesis.json
This is the production genesis state. On node initialization, the content will be 
read and used to populate the genesis file. This is the place you want to add any
initial state before blockchain launch.

### genesis_dev.json
This file is like genesis.json except it is used in development mode. Use this to describe
initial app state when in development mode. 

### Genesis File Schema
TODO: Finalize the schema and document it 

### dev_account_key

This file contains the private key of a development user account containing initial 
supply of the native coin. You can access this key from console by running `dev.devAccountKey`.
WARNING: DO NOT USE THIS KEY ON THE MAINNET. EVERYONE CAN SEE IT AND USE IT.