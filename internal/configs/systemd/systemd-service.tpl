[Unit]
Description={{.Description}}
After=network.target

[Service]
Type=simple
Environment={{.Environment}}
ExecStart={{.ExecStart}}
WorkingDirectory={{.Workingdirectory}}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
Alias=ddns6.service