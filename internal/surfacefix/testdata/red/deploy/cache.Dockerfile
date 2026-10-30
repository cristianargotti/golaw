# valkey/valkey:8.1
FROM valkey/valkey@sha256:fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210
COPY cache.conf /etc/cache.conf
ENTRYPOINT ["valkey-server", "/etc/cache.conf"]
