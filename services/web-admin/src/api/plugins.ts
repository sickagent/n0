import { api } from './client';


export interface RegisterPluginPayload {
  plugin_type: string;
  name: string;
  version: string;
  endpoint: string;
  protocol: string;
  tenant_id: string;
  global: boolean;
}

export interface RegisterPluginResponse {
  plugin_id: string;
  status: string;
}

export const pluginsApi = {
  register: async (payload: RegisterPluginPayload) => {
    const { data } = await api.post<RegisterPluginResponse>('/v1/plugins/register', payload);
    return data;
  },
};
