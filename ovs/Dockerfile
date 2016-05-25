FROM ubuntu

ENV build_deps \
    build-essential \
    libssl-dev \
    python2.7 \
    wget

ENV ver 2.5.0

WORKDIR /tmp/ovs

RUN \
apt-get update \
&& apt-get install -y ${build_deps} \
&& wget http://openvswitch.org/releases/openvswitch-${ver}.tar.gz \
&& tar -xf openvswitch-${ver}.tar.gz \
&& cd openvswitch-${ver} \
&& ./configure --localstatedir="/var" --sysconfdir="/etc" --prefix="/usr" --enable-ssl \
&& make install \
&& apt-get remove --purge -y ${build_deps} \
&& apt-get autoremove -y --purge \
&& apt-get install -y --no-install-recommends openssl ca-certificates \
&& rm -rf /var/lib/apt/lists/* \
&& rm -rf /tmp/ovs \
&& ovsdb-tool create /etc/openvswitch/conf.db /usr/share/openvswitch/vswitch.ovsschema \
&& ovsdb-tool create /etc/openvswitch/ovnnb.db /usr/share/openvswitch/ovn-nb.ovsschema \
&& ovsdb-tool create /etc/openvswitch/ovnsb.db /usr/share/openvswitch/ovn-sb.ovsschema

COPY run /bin/run

VOLUME ["/var/log/openvswitch", "/var/lib/openvswitch", "/var/run/openvswitch", "/etc/openvswitch"]
ENTRYPOINT ["run"]
