module.exports = {
  apps: [
    {
      name: 'claude-monitor-backend',
      script: './claude-monitor',
      args: '-config config.json',
      cwd: '/root/claude-monitor',
      instances: 1,
      exec_mode: 'fork',
      env: {
        NODE_ENV: 'production',
      },
      error_file: '/var/log/pm2/claude-monitor-backend-error.log',
      out_file: '/var/log/pm2/claude-monitor-backend-out.log',
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      watch: false,
      ignore_watch: ['node_modules', 'dist', '.git'],
      max_memory_restart: '1G',
      autorestart: true,
      max_restarts: 10,
      min_uptime: '10s',
    },
  ],
};
