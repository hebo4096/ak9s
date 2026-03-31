# AK9S

A k9s-like TUI tool for managing Azure Kubernetes Service (AKS) clusters.

```
    _    _  ___   ____
   / \  | |/ / _ \/ ___|
  / _ \ | ' / (_) \___ \
 / ___ \| . \\__, |___) |  Manage your AKS Clusters
/_/   \_\_|\_\  /_/|____/ 
```

## Features

- List AKS clusters across Azure subscriptions
- View detailed cluster information (General, Network, Node Pools, Tags, Addons, Extensions)
- Start / Stop / Delete clusters with commands
- Bulk operations (`/stop /bulk`, `/delete /bulk`)
- Tab completion for commands and cluster names
- Auto-refresh every 30 seconds
- Delete confirmation prompt

## Prerequisites

- Go 1.25.8
- Azure CLI (`az login`) or other Azure credentials configured via [DefaultAzureCredential](https://learn.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication)

## Installation

### From source (go install)

```sh
go install github.com/hebo4096/ak9s@latest
```

### Build from source

```sh
git clone https://github.com/hebo4096/ak9s.git
cd ak9s
go build -o ak9s ./main.go
```

### Move to PATH (optional)

```sh
sudo mv ak9s /usr/local/bin/
```

## Usage

```sh
ak9s
```

### Commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/start NAME/RG` | Start a cluster |
| `/stop NAME/RG` | Stop a cluster |
| `/delete NAME/RG` | Delete a cluster (with confirmation) |
| `/stop /bulk` | Stop all clusters |
| `/delete /bulk` | Delete all clusters (with confirmation) |

Multiple targets can be specified: `/start NAME1/RG1 NAME2/RG2`

Use `Tab` for command and cluster name completion.

## License

MIT
