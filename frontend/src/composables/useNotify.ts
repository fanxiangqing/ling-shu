import type { MessageApi } from 'naive-ui'

let messageApi: MessageApi | null = null

export function setMessageApi(api: MessageApi) {
  messageApi = api
}

export const notify = {
  success(content: string) {
    messageApi?.success(content)
  },
  error(content: string) {
    messageApi?.error(content)
  },
  warning(content: string) {
    messageApi?.warning(content)
  },
  info(content: string) {
    messageApi?.info(content)
  }
}
