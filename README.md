# TrueNAS Scale VM Setup for Dune Awakening Server

## Prerequisites
- TrueNAS Scale installed
- `dune-server.qcow2` file (200GB, converted from VHDX)
- Network configured for VM bridging
- SSH access to TrueNAS

## Step 1: Upload Disk Image to TrueNAS

Upload the qcow2 disk image to TrueNAS:

```bash
# Create directory on TrueNAS
ssh truenas "sudo mkdir -p /mnt/YOUR_POOL/virtual-disks"

# Upload the disk image (this will take a while - it's 200GB)
scp "Virtual Hard Disks/dune-server.qcow2" truenas:/mnt/YOUR_POOL/virtual-disks/
```

**Note:** Scripts (`battlegroup.sh`, `initial-setup.sh`, etc.) run from your local Mac/PC, not from TrueNAS.

## Step 2: Convert QCOW2 to Zvol

TrueNAS Scale requires a zvol for VM disks. Convert the qcow2:

```bash
# SSH into TrueNAS and run:

# Create a 200GB zvol
sudo zfs create -V 200G YOUR_POOL/virtual-disks/dune-awakening-disk

# Convert qcow2 to raw and write to zvol (takes a few minutes)
sudo qemu-img convert -p -O raw \
    /mnt/YOUR_POOL/virtual-disks/dune-server.qcow2 \
    /dev/zvol/YOUR_POOL/virtual-disks/dune-awakening-disk
```

## Step 3: Create the VM in TrueNAS UI

1. Go to **Virtualization > Virtual Machines > Add**

### General
| Setting | Value |
|---------|-------|
| Name | `DuneAwakening` (no hyphens allowed) |
| Description | Dune Awakening Private Server |
| System Clock | Local |
| Boot Method | UEFI |
| Shutdown Timeout | 90 |
| Start on Boot | Yes (recommended) |
| Enable VNC | Yes |

### CPU and Memory
| Setting | Value |
|---------|-------|
| vCPUs | 4 (minimum) |
| Cores | 2 |
| Threads | 2 |
| Memory | 20480 MB (20GB minimum) |

**Memory recommendations:**
- 20GB - Hagga Basin Sietch only
- 30GB - Hagga Basin + Story/Social maps  
- 40GB - Full server (Deep Desert included)

### Disks
1. Click **Add** under Disks
2. Select **Use existing zvol**
3. Select: `YOUR_POOL/virtual-disks/dune-awakening-disk`
4. Disk Mode: VirtIO (recommended)

### Network
1. Click **Add** under Network Interfaces
2. Adapter Type: **VirtIO**
3. NIC to attach: Select your bridge interface
4. **Important**: Use bridged mode so the VM gets its own IP on your LAN

## Step 4: Start the VM and Find IP

1. Click **Save**
2. Select the VM and click **Start**
3. Click **VNC** to watch the boot process
4. Wait for Alpine Linux to boot and get a DHCP address

**Find the IP:**

Option A - From VNC console:
```bash
ip addr show eth0
```

Option B - Check your router's DHCP leases

Option C - From TrueNAS (if virsh available):
```bash
sudo virsh domifaddr DuneAwakening
```

## Step 5: Test SSH Access

```bash
# Set the VM IP
export DUNE_VM_IP=192.168.1.XXX

# Test SSH (from your Mac/PC with the scripts)
ssh -i truenas/sshKey dune@$DUNE_VM_IP
```

## Running Scripts

**Important:** Run all scripts from your local Mac/PC, not from TrueNAS. The TrueNAS host may not have network routing to the VM's bridged IP.

Local scripts location: `~/dune-scripts/` (or the `truenas/` folder in this repo)

```
~/dune-scripts/
├── initial-setup.sh    # First-time setup (run once)
├── bootstrap-setup     # Uploaded to VM by initial-setup
├── battlegroup.sh      # Server management (start/stop/status)
└── sshKey              # SSH key for dune user
```

## Step 6: Run Initial Setup (First Time Only)

Run the initial setup script from your Mac/PC (not TrueNAS - network routing may differ):

```bash
cd truenas/
./initial-setup.sh $DUNE_VM_IP
```

This will:
1. Write the settings.conf with the VM IP
2. Upload the bootstrap script
3. Run Steam download (~20GB via SteamCMD anonymous login)
4. Set up k3s and the battlegroup operator

**Note:** The Steam download may take 10-30 minutes depending on your internet speed.

### If Steam Download Fails

If you see error 0x202 or the download fails:

```bash
# SSH into the VM
ssh -i truenas/sshKey dune@$DUNE_VM_IP

# Retry manually with validation
steamcmd +force_install_dir /home/dune/.dune/download +login anonymous +app_update 3104830 validate +quit

# Or run the setup script again
/home/dune/.dune/bin/setup
```

## Step 7: Manage with battlegroup.sh

Run from your Mac/PC (where you have the scripts):

```bash
# Set the IP
export DUNE_VM_IP=192.168.1.XXX

# Run the management script (interactive menu)
./truenas/battlegroup.sh

# Or run specific commands directly:
DUNE_VM_IP=192.168.1.XXX ./truenas/battlegroup.sh status
DUNE_VM_IP=192.168.1.XXX ./truenas/battlegroup.sh start
DUNE_VM_IP=192.168.1.XXX ./truenas/battlegroup.sh stop
```

**Tip:** Add to your shell profile for convenience:
```bash
echo 'export DUNE_VM_IP=192.168.1.XXX' >> ~/.zshrc
echo 'alias dune="~/path/to/truenas/battlegroup.sh"' >> ~/.zshrc
```

Then just run: `dune start`, `dune status`, etc.

## Network Ports

Ensure these ports are accessible (forward from router if needed):

| Port | Service |
|------|---------|
| 22 | SSH (management) |
| 11717 | Director (dynamic NodePort) |
| 18888 | File Browser |
| 7777-7780 | Game server (UDP) |

## Static IP Configuration (Recommended)

For consistent access, set a static IP:

```bash
ssh -i truenas/sshKey dune@$DUNE_VM_IP
sudo vi /etc/network/interfaces
```

Edit to:
```
auto lo
iface lo inet loopback

auto eth0
iface eth0 inet static
    address 192.168.1.100/24
    gateway 192.168.1.1
```

Then:
```bash
sudo rc-service networking restart
```

## Troubleshooting

### VM doesn't get IP
- Check bridge configuration in TrueNAS
- Ensure DHCP server is on your network
- Try VirtIO vs AHCI for network adapter

### SSH connection refused
- VM may still be booting - wait 60 seconds
- Check if SSH is running: `rc-service sshd status` (via VNC)

### k3s/kubectl not working
- Wait for k3s to initialize (can take 2-3 minutes after boot)
- Check: `sudo systemctl status k3s` or `sudo rc-service k3s status`

### Battlegroup won't start
- Check logs: `./truenas/battlegroup.sh logs-export`
- Verify memory allocation is sufficient
- Run `./truenas/battlegroup.sh update` to ensure latest version
