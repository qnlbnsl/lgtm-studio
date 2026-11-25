import type { ServiceStarting } from '@lgtm/contracts/client/startup'

export const startingNotice = (state: ServiceStarting): string =>
  `The service is starting. ${state.recovered} of ${state.total} recovery attempts finished.`
