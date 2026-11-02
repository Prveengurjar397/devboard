import { useQuery } from '@tanstack/react-query';
import { organizationApi } from '../api/organizations';

export function useOrganizationContext() {
  const query = useQuery({
    queryKey: ['organizations'],
    queryFn: organizationApi.list,
    staleTime: 30000,
    retry: 1,
  });

  const organizations = query.data?.organizations || [];
  const isPlatformAdmin = organizations.some(
    (organization) => organization.role === 'platform_admin'
  );

  return {
    ...query,
    organizations,
    isPlatformAdmin,
    currentOrganization: organizations[0] || null,
    canManageMembers: organizations.some(
      (organization) =>
        organization.role === 'company_admin' ||
        organization.role === 'platform_admin'
    ),
  };
}
