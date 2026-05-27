# Run unit tests during package build
%global with_check 1
# Include rhcd->yggdrasil compatibility patch
%global with_rhcd_compat 0

%global goipath         github.com/redhatinsights/rhc
Version:                0.3.9

%gometa -L -f

Name:           rhc
Release:        %autorelease
Summary:        Tool for registering to Red Hat services
License:        Apache-2.0 AND BSD-2-Clause AND BSD-3-Clause AND GPL-3.0-only AND MIT
URL:            %{gourl}
Source0:        %{gosource}
Source1:        %{archivename}-vendor.tar.bz2
Source2:        go-vendor-tools.toml

BuildRequires:  go-vendor-tools
BuildRequires:  systemd-rpm-macros
BuildRequires:  askalono-cli
%if 0%{?with_check}
BuildRequires:  /usr/bin/dbus-launch
%endif

Requires: subscription-manager
# insights-client only exists in CentOS Stream and RHEL
%if ! 0%{?fedora}
Requires: insights-client
%endif
Requires: yggdrasil >= 0.4
Requires: yggdrasil-worker-package-manager

%description
Client tool to register Fedora, CentOS Stream or Red Hat Enterprise Linux
to Red Hat Subscription Management and Red Hat Lightspeed.

%prep
%goprep -A
%setup -q -T -D -a1 %{forgesetupargs}

%generate_buildrequires
%go_vendor_license_buildrequires -c %{S:2}

%build
export GO_LDFLAGS="-X github.com/redhatinsights/rhc/pkg/version.Version=%{version} -X main.ServiceName=yggdrasil"
%gobuild -o %{gobuilddir}/bin/rhc %{goipath}/cmd/rhc
%gobuild -o %{gobuilddir}/bin/rhc-server %{goipath}/cmd/rhc-server
%gobuild -o %{gobuilddir}/bin/rhc-collector %{goipath}/cmd/rhc-collector

# Generate man page
%{gobuilddir}/bin/rhc --generate-man-page > rhc.1

%install
# Licenses
%go_vendor_license_install -c %{S:2}
# Binaries
install -m 0755 -vd                     %{buildroot}%{_bindir}
install -m 0755 -vp _build/bin/rhc      %{buildroot}%{_bindir}/
install -m 0755 -vd                     %{buildroot}%{_libexecdir}/%{name}
install -m 0755 -vp _build/bin/rhc-server %{buildroot}%{_libexecdir}/%{name}/
install -m 0755 -vp _build/bin/rhc-collector %{buildroot}%{_libexecdir}/%{name}/
# Bash completion
install -m 0755 -vd                     %{buildroot}%{bash_completions_dir}/
install -m 0644 -vp data/completion/rhc.bash  %{buildroot}%{bash_completions_dir}/%{name}
# Logs
install -m 0755 -vd                     %{buildroot}%{_localstatedir}/log/%{name}/
# Logrotate
install -m 0755 -vd                     %{buildroot}%{_sysconfdir}/logrotate.d/
install -m 0644 -vp data/logrotate.d/rhc %{buildroot}%{_sysconfdir}/logrotate.d/rhc
# Man page
install -m 0755 -vd                     %{buildroot}%{_mandir}/man1
install -m 0644 -vp rhc.1               %{buildroot}%{_mandir}/man1/rhc.1
# Systemd files
install -m 0755 -vd                     %{buildroot}%{_unitdir}
install -m 0644 -vp data/systemd/rhc-canonical-facts.*  %{buildroot}%{_unitdir}/
install -m 0644 -vp data/systemd/rhc-server.service  %{buildroot}%{_unitdir}/
install -m 0644 -vp data/systemd/rhc-server.socket   %{buildroot}%{_unitdir}/
install -m 0755 -vd %{buildroot}%{_prefix}/lib/systemd/system-preset/
install -m 0644 -vp data/systemd/presets/50-rhc.preset %{buildroot}%{_prefix}/lib/systemd/system-preset/
# Configuration
install -m 0755 -vd                     %{buildroot}%{_sysconfdir}/%{name}/
# Yggdrasil
%if 0%{?with_rhcd_compat}
install -m 0755 -vd %{buildroot}%{_unitdir}/yggdrasil.service.d/
install -m 0644 -vp %{buildroot}%{_unitdir}/yggdrasil.service.d/rhcd.conf %{buildroot}%{_unitdir}/yggdrasil.service.d/
%endif

%check
%go_vendor_license_check -c %{S:2}
%if 0%{?with_check}
%gocheck2
%endif

%post
%systemd_post rhc-canonical-facts.timer
%systemd_post rhc-server.socket
if [ "$1" -eq 1 ] ; then
    systemctl start rhc-server.socket >/dev/null 2>&1 || :
fi
%if 0%{?with_rhcd_compat}
# On package update, ensure yggdrasil (formerly rhcd) has its own configuration file
if [ $1 -eq 2 ] && [ ! -f /etc/yggdrasil/config.toml ]; then
	cp /etc/rhc/config.toml /etc/yggdrasil/config.toml.migrated
	sed -E 's#^broker( ?=)#server\1#' /etc/yggdrasil/config.toml.migrated > /etc/yggdrasil/config.toml
	echo 'facts-file = "/var/lib/yggdrasil/canonical-facts.json"' >> /etc/yggdrasil/config.toml
	rm /etc/yggdrasil/config.toml.migrated
fi
%endif

%preun
%systemd_preun rhc-canonical-facts.timer
%systemd_preun rhc-server.socket rhc-server.service

%postun
%systemd_postun_with_restart rhc-canonical-facts.timer
%systemd_postun_with_restart rhc-server.service

%files -f %{go_vendor_license_filelist}
# Binaries
%{_bindir}/rhc
%{_libexecdir}/%{name}/rhc-server
%{_libexecdir}/%{name}/rhc-collector
# Bash completion
%{bash_completions_dir}/%{name}
# Man page
%{_mandir}/man1/*
# Systemd
%{_unitdir}/rhc-canonical-facts.*
%{_unitdir}/rhc-server.service
%{_unitdir}/rhc-server.socket
%{_prefix}/lib/systemd/system-preset/50-rhc.preset
# Configuration
%{_sysconfdir}/%{name}/
# Logrotate
%config(noreplace) %{_sysconfdir}/logrotate.d/rhc
# Yggdrasil
%if 0%{?with_rhcd_compat}
%{_unitdir}/yggdrasil.service.d/rhcd.conf
%endif

%changelog
%autochangelog
