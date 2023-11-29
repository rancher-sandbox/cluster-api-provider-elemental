#
# spec file for package elemental-agent
#
# Copyright (c) 2023 SUSE LLC
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (unless the
# license for the pristine package is not an Open Source License, in which
# case the license is the MIT License). An "Open Source License" is a
# license that conforms to the Open Source Definition (Version 1.9)
# published by the Open Source Initiative.

# Please submit bugfixes or comments via https://bugs.opensuse.org/
#

%define pluginsdir /usr/lib/elemental/plugins

Name:           elemental-agent
Version:        0
Release:        0
Summary:        Elemental CAPI agent
License:        Apache-2.0
Group:          System/Management
URL:            https://github.com/rancher-sandbox/cluster-api-provider-elemental
Source:         %{name}-%{version}.tar
Source1:        %{name}.obsinfo
Requires:       elemental-plugin

BuildRequires:  make

BuildRequires:  golang(API) >= 1.21
BuildRequires:  golang-packaging
%{go_provides}

BuildRoot:      %{_tmppath}/%{name}-%{version}-build

%description
The Elemental CAPI agent is responsible for managing the OS 
versions and maintaining a machine inventory to assist with edge or 
baremetal installations.

%package -n elemental-systemd-services
Summary: Elemental CAPI agent systemd services
Requires: elemental-agent = %{version}-%{release}
Requires: elemental-plugin-toolkit = %{version}-%{release}
%{?systemd_requires}
%description -n elemental-systemd-services
This package contains systemd services to run the elemental-agent 
when the elemental-plugin-toolkit is also in use.

%package -n elemental-plugin-toolkit
Summary: elemental-toolkit plugin
Provides: elemental-plugin
Requires: elemental-agent = %{version}-%{release}
Requires: elemental-toolkit
%description -n elemental-plugin-toolkit
The toolkit plugin allows integration between the elemental-toolkit 
and the elemental-agent.

%package -n elemental-plugin-dummy
Summary: dummy plugin
Provides: elemental-plugin
Requires: elemental-agent = %{version}-%{release}
%description -n elemental-plugin-dummy
The dummy plugin is a very basic plugin for the elemental-agent 
that can be used for debugging, or when no other plugin option 
is available.

%prep
%setup -q -n %{name}-%{version}
cp %{S:1} .

%build
%goprep .

export GIT_TAG=`echo "%{version}" | cut -d "+" -f 1`
GIT_COMMIT=$(cat %{name}.obsinfo | grep commit: | cut -d" " -f 2)
export GIT_COMMIT=${GIT_COMMIT:0:8}
MTIME=$(cat %{name}.obsinfo | grep mtime: | cut -d" " -f 2)
export GIT_COMMIT_DATE=$(date -d @${MTIME} +%Y%m%d)

mkdir -p bin
make build-agent
make build-plugins

%install
%goinstall

%{__install} -d -m 755 %{buildroot}%{_sbindir}
%{__install} -d -m 755 %{buildroot}%{pluginsdir}

%{__install} -m 755 bin/elemental-agent %{buildroot}%{_sbindir}
%{__install} -m 755 bin/elemental.so %{buildroot}%{pluginsdir}
%{__install} -m 755 bin/dummy.so %{buildroot}%{pluginsdir}

mkdir -p %{buildroot}%{_unitdir}
cp -a framework/files/usr/lib/systemd/system/* %{buildroot}%{_unitdir}
%pre -n elemental-systemd-services
%service_add_pre elemental-agent.service
%service_add_pre elemental-agent-install.service

%post -n elemental-systemd-services
%service_add_post elemental-agent.service
%service_add_post elemental-agent-install.service

%preun -n elemental-systemd-services
%service_del_preun elemental-agent.service
%service_del_preun elemental-agent-install.service

%postun -n elemental-systemd-services
%service_del_postun elemental-agent.service
%service_del_postun elemental-agent-install.service

%files
%defattr(-,root,root,-)
%license LICENSE
%{_sbindir}/%{name}
%dir %{pluginsdir}

%files -n elemental-systemd-services
%defattr(-,root,root,-)
%license LICENSE
%{_unitdir}/elemental-agent.service
%{_unitdir}/elemental-agent-install.service

%files -n elemental-plugin-toolkit
%defattr(-,root,root,-)
%license LICENSE
%{pluginsdir}/elemental.so

%files -n elemental-plugin-dummy
%defattr(-,root,root,-)
%license LICENSE
%{pluginsdir}/dummy.so

%changelog
