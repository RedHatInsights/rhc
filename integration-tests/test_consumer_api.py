"""
:casecomponent: rhc
:requirement: RHSS-XXXXX
:subsystemteam: rhel-sst-csi-client-tools
:caseautomation: Automated
:upstream: Yes
"""

import uuid

import pytest

from conftest import CONSUMER_API
from utils.varlink import run_varlinkctl


pytestmark = pytest.mark.usefixtures("rhc_server_socket")


@pytest.mark.tier2
class TestConsumerAPIRegistered:
    """Tests for the consumer API when the system is registered."""

    @pytest.fixture(autouse=True)
    def _require_registered(self):
        """Skip tests in this class if the system is not registered."""
        response = run_varlinkctl("com.redhat.rhsm.testing.IsRegistered")
        if not response.get("registered", False):
            pytest.skip("System is not registered with RHSM")

    def test_get_uuid(self):
        """
        :id: a1000001-0001-4000-a001-000000000001
        :title: Verify GetUUID returns a valid UUID string
        :description:
            Test that the com.redhat.rhc.testing.rhsm.consumer.GetUUID method
            returns a valid UUID when the system is registered.
        :tags: Tier 2
        :steps:
            1. Call com.redhat.rhc.testing.rhsm.consumer.GetUUID via varlinkctl
            2. Verify the response contains a "uuid" field
            3. Verify the UUID is a non-empty string
        :expectedresults:
            1. The varlink call succeeds
            2. Response contains "uuid" key
            3. UUID is a non-empty string in standard UUID format
        """
        response = run_varlinkctl(f"{CONSUMER_API}.GetUUID")

        assert "uuid" in response
        assert isinstance(response["uuid"], str)
        uuid.UUID(response["uuid"])

    def test_get_organization(self):
        """
        :id: a1000001-0001-4000-a001-000000000002
        :title: Verify GetOrganization returns organization data
        :description:
            Test that the com.redhat.rhc.testing.rhsm.consumer.GetOrganization
            method returns organization information for a registered system.
        :tags: Tier 2
        :steps:
            1. Call com.redhat.rhc.testing.rhsm.consumer.GetOrganization via varlinkctl
            2. Verify the response contains an "org" field
            3. Verify the org field is a JSON object with a "key" field
        :expectedresults:
            1. The varlink call succeeds
            2. Response contains "org" key
            3. Org object contains a non-empty "key" field
        """
        response = run_varlinkctl(f"{CONSUMER_API}.GetOrganization")

        assert "org" in response
        assert isinstance(response["org"], dict)
        assert "key" in response["org"]
        assert isinstance(response["org"]["key"], str)
        assert len(response["org"]["key"]) > 0
        assert "id" in response["org"]
        assert isinstance(response["org"]["id"], str)
        assert len(response["org"]["id"]) > 0

    def test_get_environments(self):
        """
        :id: a1000001-0001-4000-a001-000000000003
        :title: Verify GetEnvironments returns environment list
        :description:
            Test that the com.redhat.rhc.testing.rhsm.consumer.GetEnvironments
            method returns environment information for a registered system.
        :tags: Tier 2
        :steps:
            1. Call com.redhat.rhc.testing.rhsm.consumer.GetEnvironments via varlinkctl
            2. Verify the response contains an "environments" field
            3. Verify the environments value is a JSON array
        :expectedresults:
            1. The varlink call succeeds
            2. Response contains "environments" key
            3. Environments is a valid JSON array
        """
        response = run_varlinkctl(f"{CONSUMER_API}.GetEnvironments")

        assert "environments" in response
        assert isinstance(response["environments"], list)
        if response["environments"]:
            first = response["environments"][0]
            assert isinstance(first, dict)
            assert "id" in first
            assert isinstance(first["id"], str)
            assert len(first["id"]) > 0


@pytest.mark.tier2
class TestConsumerAPIUnregistered:
    """Tests for the consumer API when the system is NOT registered."""

    @pytest.fixture(autouse=True)
    def _require_unregistered(self):
        """Skip tests in this class if the system IS registered."""
        response = run_varlinkctl("com.redhat.rhsm.testing.IsRegistered")
        if response.get("registered", False):
            pytest.skip("System is registered with RHSM (need unregistered)")

    def test_get_uuid_returns_system_not_registered(self):
        """
        :id: a1000001-0001-4000-a001-000000000005
        :title: Verify GetUUID returns SystemNotRegistered error when unregistered
        :description:
            Test that calling GetUUID on an unregistered system returns
            the SystemNotRegistered error.
        :tags: Tier 2
        :steps:
            1. Ensure system is not registered
            2. Call com.redhat.rhc.testing.rhsm.consumer.GetUUID
            3. Verify the call fails with SystemNotRegistered error
        :expectedresults:
            1. System is confirmed unregistered
            2. The varlink call fails
            3. Error indicates SystemNotRegistered
        """
        result = run_varlinkctl(f"{CONSUMER_API}.GetUUID", check=False)

        assert result.returncode != 0
        assert "SystemNotRegistered" in result.stderr

    def test_get_organization_returns_system_not_registered(self):
        """
        :id: a1000001-0001-4000-a001-000000000006
        :title: Verify GetOrganization returns SystemNotRegistered when unregistered
        :description:
            Test that calling GetOrganization on an unregistered system
            returns the SystemNotRegistered error.
        :tags: Tier 2
        :steps:
            1. Ensure system is not registered
            2. Call com.redhat.rhc.testing.rhsm.consumer.GetOrganization
            3. Verify the call fails with SystemNotRegistered error
        :expectedresults:
            1. System is confirmed unregistered
            2. The varlink call fails
            3. Error indicates SystemNotRegistered
        """
        result = run_varlinkctl(
            f"{CONSUMER_API}.GetOrganization", check=False
        )

        assert result.returncode != 0
        assert "SystemNotRegistered" in result.stderr

    def test_get_environments_returns_system_not_registered(self):
        """
        :id: a1000001-0001-4000-a001-000000000007
        :title: Verify GetEnvironments returns SystemNotRegistered when unregistered
        :description:
            Test that calling GetEnvironments on an unregistered system
            returns the SystemNotRegistered error.
        :tags: Tier 2
        :steps:
            1. Ensure system is not registered
            2. Call com.redhat.rhc.testing.rhsm.consumer.GetEnvironments
            3. Verify the call fails with SystemNotRegistered error
        :expectedresults:
            1. System is confirmed unregistered
            2. The varlink call fails
            3. Error indicates SystemNotRegistered
        """
        result = run_varlinkctl(
            f"{CONSUMER_API}.GetEnvironments", check=False
        )

        assert result.returncode != 0
        assert "SystemNotRegistered" in result.stderr
