[Unit]
Description={{.Description}}
After=network.target

[Service]
Type=simple
Environment={{.Environment}}
ExecStart={{.ExecStart}}
WorkingDirectory={{.WorkingDirectory}}
Restart=always

[Install]
WantedBy=multi-user.target
Alias=ddns6.service