import { DecoratorFn, Meta, Story } from '@storybook/react'
import classNames from 'classnames'

import { Container } from '@sourcegraph/wildcard'

import { WebStory } from '../../components/WebStory'
import { WebhookLogFields } from '../../graphql-operations'

import { webhookLogNode } from './story/fixtures'
import { WebhookLogNode } from './WebhookLogNode'

import gridStyles from './WebhookLogPage.module.scss'

const decorator: DecoratorFn = story => (
    <Container>
        <div className={classNames('p-3', 'container', gridStyles.logs)}>{story()}</div>
    </Container>
)

const config: Meta = {
    title: 'web/site-admin/webhooks/WebhookLogNode',
    parameters: {
        chromatic: {
            viewports: [320, 576, 978, 1440],
        },
    },
    decorators: [decorator],
    argTypes: {
        receivedAt: {
            name: 'received at',
            control: { type: 'text' },
            defaultValue: 'Sun Nov 07 2021 14:31:00 GMT-0500 (Eastern Standard Time)',
        },
        statusCode: {
            name: 'status code',
            control: { type: 'number', min: 100, max: 599 },
            defaultValue: 204,
        },
    },
}

export default config

// Most of the components of WebhookLogNode are more thoroughly tested elsewhere
// in the storybook, so this is just a limited number of cases to ensure the
// expando behaviour is correct, the date formatting does something useful, and
// the external service name is handled properly when there isn't an external
// service.
//
// Some bonus controls are provided for the tinkerers.

type StoryArguments = Pick<WebhookLogFields, 'receivedAt' | 'statusCode'>

export const Collapsed: Story<StoryArguments> = args => (
    <WebStory>
        {() => (
            <WebhookLogNode
                node={webhookLogNode(args, {
                    externalService: {
                        displayName: 'GitLab',
                    },
                })}
            />
        )}
    </WebStory>
)

export const ExpandedRequest: Story<StoryArguments> = args => (
    <WebStory>{() => <WebhookLogNode node={webhookLogNode(args)} initiallyExpanded={true} />}</WebStory>
)

ExpandedRequest.storyName = 'expanded request'

export const ExpandedResponse: Story<StoryArguments> = args => (
    <WebStory>
        {() => <WebhookLogNode node={webhookLogNode(args)} initiallyExpanded={true} initialTabIndex={1} />}
    </WebStory>
)

ExpandedResponse.storyName = 'expanded response'
