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

BuildRequires:  gcc
BuildRequires:  make

%if 0%{?suse_version}
BuildRequires:  golang(API) >= 1.21
BuildRequires:  golang-packaging
%{go_provides}
%else
%global goipath    google.golang.org/api
%global forgeurl   https://github.com/rancher-sandbox/cluster-api-provider-elemental
%global commit     25abcdc57b9409d4c5b2009cf0a2f9aa6ff647ad
%gometa
%if (0%{?centos_version} == 800) || (0%{?rhel_version} == 800)
BuildRequires:  go1.21
%else
BuildRequires:  compiler(go-compiler)
%endif
%endif

BuildRoot:      %{_tmppath}/%{name}-%{version}-build

%description
The Elemental CAPI agent is responsible for managing the OS
versions and maintaining a machine inventory to assist with edge or
baremetal installations.

%prep
%setup -q -n %{name}-%{version}
cp %{S:1} .

%build
%if 0%{?suse_version}
%goprep .
%endif

export GIT_TAG=`echo "%{version}" | cut -d "+" -f 1`
GIT_COMMIT=$(cat %{name}.obsinfo | grep commit: | cut -d" " -f 2)
export GIT_COMMIT=${GIT_COMMIT:0:8}
MTIME=$(cat %{name}.obsinfo | grep mtime: | cut -d" " -f 2)
export COMMITDATE=$(date -d @${MTIME} +%Y%m%d)

mkdir -p bin
make build-agent
make build-plugins

%install
%if 0%{?suse_version}
%goinstall
%endif

%{__install} -d -m 755 %{buildroot}/%{_sbindir}
%{__install} -d -m 755 %{buildroot}/%{_pluginsdir}

%{__install} -m 755 bin/elemental-agent %{buildroot}%{_sbindir}
%{__install} -m 755 bin/elemental.so %{buildroot}%{_pluginsdir}
%{__install} -m 755 bin/dummy.so %{buildroot}%{_pluginsdir}


cp -a framework/files/* %{buildroot}
%pre
%if 0%{?suse_version}
%service_add_pre elemental-agent.service
%service_add_pre elemental-agent-install.service
%endif

%post
%if 0%{?suse_version}
%service_add_post elemental-agent.service
%service_add_post elemental-agent-install.service
%else
%systemd_post elemental-agent.service
%systemd_post elemental-agent-install.service
%endif

%preun
%if 0%{?suse_version}
%service_del_preun elemental-agent.service
%service_del_preun elemental-agent-install.service
%else
%systemd_preun elemental-agent.service
%systemd_preun elemental-agent-install.service
%endif

%postun
%if 0%{?suse_version}
%service_del_postun elemental-agent.service
%service_del_postun elemental-agent-install.service
%else
%systemd_postun elemental-agent.service
%systemd_postun elemental-agent-install.service
%endif

%files
%defattr(-,root,root,-)
%license LICENSE
%{_sbindir}/%{name}

%changelog
