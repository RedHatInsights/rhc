# Run unit tests during package build
%global with_check 1
# Include rhcd->yggdrasil compatibility patch
%global with_rhcd_compat 0

%global goipath         github.com/redhatinsights/rhc
Version:                0.3.8

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
export GO_LDFLAGS="-X main.Version=%{version} -X main.ServiceName=yggdrasil"
%gobuild -o %{gobuilddir}/bin/rhc %{goipath}/cmd/rhc

# Generate man page
%{gobuilddir}/bin/rhc --generate-man-page > rhc.1

%install
%go_vendor_license_install -c %{S:2}
# Binaries
install -m 0755 -vd                     %{buildroot}%{_bindir}
install -m 0755 -vp _build/bin/*        %{buildroot}%{_bindir}/
# Bash completion
install -m 0755 -vd                     %{buildroot}%{bash_completions_dir}/
install -m 0644 -vp data/completion/rhc.bash  %{buildroot}%{bash_completions_dir}/%{name}
# Man page
install -m 0755 -vd                     %{buildroot}%{_mandir}/man1
install -m 0644 -vp rhc.1               %{buildroot}%{_mandir}/man1/rhc.1
# Systemd files
install -m 0755 -vd                     %{buildroot}%{_unitdir}
install -m 0644 -vp data/systemd/rhc-canonical-facts.*  %{buildroot}%{_unitdir}/
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

%postun
%systemd_postun_with_restart rhc-canonical-facts.timer

%files -f %{go_vendor_license_filelist}
# Binaries
%{_bindir}/rhc
# Bash completion
%{bash_completions_dir}/%{name}
# Man page
%{_mandir}/man1/*
# Systemd
%{_unitdir}/rhc-canonical-facts.*
# Configuration
%{_sysconfdir}/%{name}/
# Yggdrasil
%if 0%{?with_rhcd_compat}
%{_unitdir}/yggdrasil.service.d/rhcd.conf
%endif

%changelog
%autochangelog
