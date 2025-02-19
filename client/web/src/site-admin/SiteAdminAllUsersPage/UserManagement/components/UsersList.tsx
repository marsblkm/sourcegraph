import React, { useState, useCallback } from 'react'

import {
    mdiLogoutVariant,
    mdiArchive,
    mdiDelete,
    mdiLockReset,
    mdiClipboardMinus,
    mdiClipboardPlus,
    mdiClose,
} from '@mdi/js'
import classNames from 'classnames'
import { formatDistanceToNowStrict } from 'date-fns'

import { ErrorAlert } from '@sourcegraph/branded/src/components/alerts'
import { useMutation, useQuery } from '@sourcegraph/http-client'
import { H2, LoadingSpinner, Text, Input, Button, Alert, useDebounce, Link, Tooltip, Icon } from '@sourcegraph/wildcard'

import { CopyableText } from '../../../../components/CopyableText'
import {
    SiteUsersLastActivePeriod,
    SiteUserOrderBy,
    UsersManagementUsersListResult,
    UsersManagementUsersListVariables,
} from '../../../../graphql-operations'
import { useURLSyncedState } from '../../../../hooks'
import { eventLogger } from '../../../../tracking/eventLogger'
import { HorizontalSelect } from '../../../analytics/components/HorizontalSelect'
import { randomizeUserPassword, setUserIsSiteAdmin } from '../../../backend'
import { DELETE_USERS, DELETE_USERS_FOREVER, FORCE_SIGN_OUT_USERS, USERS_MANAGEMENT_USERS_LIST } from '../queries'

import { Table } from './Table'

import styles from '../index.module.scss'

type SiteUser = UsersManagementUsersListResult['site']['users']['nodes'][0]

const LIMIT = 25
interface UsersListProps {
    onActionEnd?: () => void
}

export const UsersList: React.FunctionComponent<UsersListProps> = ({ onActionEnd }) => {
    const [filters, setFilters] = useURLSyncedState({
        searchText: '',
        limit: LIMIT.toString(),
        lastActivePeriod: SiteUsersLastActivePeriod.ALL,
        orderBy: SiteUserOrderBy.EVENTS_COUNT,
        descending: 'false',
    })
    const debouncedSearchText = useDebounce(filters.searchText, 300)
    const showMore = useCallback(() => setFilters({ limit: (Number(filters.limit) + LIMIT).toString() }), [
        filters.limit,
        setFilters,
    ])

    const { data, previousData, refetch, variables, error, loading } = useQuery<
        UsersManagementUsersListResult,
        UsersManagementUsersListVariables
    >(USERS_MANAGEMENT_USERS_LIST, {
        variables: {
            first: Number(filters.limit),
            query: debouncedSearchText || null,
            lastActivePeriod: filters.lastActivePeriod,
            orderBy: filters.orderBy,
            descending: JSON.parse(filters.descending),
        },
    })

    const handleActionEnd = useCallback(
        (error?: any) => {
            if (!error) {
                // reload data
                refetch(variables).catch(console.error)
                onActionEnd?.()
            }
        },
        [onActionEnd, refetch, variables]
    )

    const {
        handleDeleteUsers,
        handleDeleteUsersForever,
        handleForceSignOutUsers,
        handleRevokeSiteAdmin,
        handlePromoteToSiteAdmin,
        handleResetUserPassword,
        notification,
        handleDismissNotification,
    } = useUserListActions(handleActionEnd)

    const users = (data || previousData)?.site.users
    return (
        <div className="position-relative">
            <div className="mb-4 mt-4 pt-4 d-flex justify-content-between align-items-center text-nowrap">
                <H2>Users</H2>
                <div className="d-flex w-75">
                    <HorizontalSelect<SiteUsersLastActivePeriod>
                        className="mr-4"
                        value={filters.lastActivePeriod}
                        label="Last active"
                        onChange={value => {
                            setFilters({ lastActivePeriod: value })
                            eventLogger.log(`UserManagementLastActive${value}`)
                        }}
                        items={[
                            { value: SiteUsersLastActivePeriod.ALL, label: 'All' },
                            { value: SiteUsersLastActivePeriod.TODAY, label: 'Today' },
                            { value: SiteUsersLastActivePeriod.THIS_WEEK, label: 'This week' },
                            { value: SiteUsersLastActivePeriod.THIS_MONTH, label: 'This month' },
                        ]}
                    />
                    <div className="flex-1 d-flex align-items-baseline m-0">
                        <Text as="label">Search users</Text>
                        <Input
                            className="flex-1 ml-2"
                            placeholder="Search username, display name, email"
                            value={filters.searchText}
                            onChange={event => setFilters({ searchText: event.target.value })}
                        />
                    </div>
                </div>
            </div>
            {notification && (
                <Alert
                    className="mt-2 d-flex justify-content-between align-items-center"
                    variant={notification.isError ? 'danger' : 'success'}
                >
                    {notification.text}
                    <Button variant="secondary" outline={true} onClick={handleDismissNotification}>
                        <Icon aria-label="Close notification" svgPath={mdiClose} />
                    </Button>
                </Alert>
            )}
            {loading && (
                <div
                    className={classNames(
                        'position-absolute w-100 h-100 d-flex justify-content-center align-items-center',
                        styles.loadingSpinnerContainer
                    )}
                >
                    <LoadingSpinner />
                </div>
            )}
            {error && <ErrorAlert error={error} />}
            {users && (
                <>
                    <Table
                        selectable={true}
                        sortBy={{ key: filters.orderBy, descending: JSON.parse(filters.descending) as boolean }}
                        data={users.nodes}
                        onSortByChange={value => {
                            setFilters({
                                orderBy: value.key as SiteUserOrderBy,
                                descending: value.descending.toString(),
                            })
                        }}
                        getRowId={({ id }) => id}
                        actions={[
                            {
                                key: 'force-sign-out',
                                label: 'Force sign-out',
                                icon: mdiLogoutVariant,
                                onClick: handleForceSignOutUsers,
                                bulk: true,
                                condition: users => users.some(user => !user.deletedAt),
                            },
                            {
                                key: 'reset-password',
                                label: 'Reset password',
                                icon: mdiLockReset,
                                onClick: handleResetUserPassword,
                                condition: ([user]) => !user?.deletedAt,
                            },
                            {
                                key: 'revoke-site-admin',
                                label: 'Revoke site admin',
                                icon: mdiClipboardMinus,
                                onClick: handleRevokeSiteAdmin,
                                condition: ([user]) => user?.siteAdmin && !user?.deletedAt,
                            },
                            {
                                key: 'promote-to-site-admin',
                                label: 'Promote to site admin',
                                icon: mdiClipboardPlus,
                                onClick: handlePromoteToSiteAdmin,
                                condition: ([user]) => !user?.siteAdmin && !user?.deletedAt,
                            },
                            {
                                key: 'delete',
                                label: 'Delete',
                                icon: mdiArchive,
                                iconColor: 'danger',
                                onClick: handleDeleteUsers,
                                bulk: true,
                                condition: users => users.some(user => !user.deletedAt),
                            },
                            {
                                key: 'delete-forever',
                                label: 'Delete forever',
                                icon: mdiDelete,
                                iconColor: 'danger',
                                labelColor: 'danger',
                                onClick: handleDeleteUsersForever,
                                bulk: true,
                            },
                        ]}
                        columns={[
                            {
                                key: SiteUserOrderBy.USERNAME,
                                accessor: 'username',
                                header: 'User',
                                sortable: true,
                                render: RenderUsernameAndEmail,
                            },
                            {
                                key: SiteUserOrderBy.SITE_ADMIN,
                                accessor: item => (item.siteAdmin ? 'Yes' : 'No'),
                                header: { label: 'Site Admin', align: 'right' },
                                sortable: true,
                                align: 'center',
                            },
                            {
                                key: SiteUserOrderBy.EVENTS_COUNT,
                                accessor: 'eventsCount',
                                header: {
                                    label: 'Events',
                                    align: 'right',
                                    tooltip:
                                        '"Events" count is cached and updated every 12 hours. It is based on event logs table and available for the last 93 days.',
                                },
                                sortable: true,
                                align: 'right',
                            },
                            {
                                key: SiteUserOrderBy.LAST_ACTIVE_AT,
                                accessor: item =>
                                    item.lastActiveAt
                                        ? formatDistanceToNowStrict(new Date(item.lastActiveAt), { addSuffix: true })
                                        : '',
                                header: {
                                    label: 'Last Active',
                                    align: 'right',
                                    tooltip:
                                        '"Last Active" is cached and updated every 12 hours. It is based on event logs table and available for the last 93 days.',
                                },
                                sortable: true,
                                align: 'right',
                            },
                            {
                                key: SiteUserOrderBy.CREATED_AT,
                                accessor: item =>
                                    formatDistanceToNowStrict(new Date(item.createdAt), { addSuffix: true }),
                                header: { label: 'Created', align: 'right' },
                                sortable: true,
                                align: 'right',
                            },
                            {
                                key: SiteUserOrderBy.DELETED_AT,
                                accessor: item =>
                                    item.deletedAt
                                        ? formatDistanceToNowStrict(new Date(item.deletedAt), { addSuffix: true })
                                        : '',
                                header: { label: 'Deleted', align: 'right' },
                                sortable: true,
                                align: 'right',
                            },
                        ]}
                        note={
                            <Text as="span">
                                {/* TODO: Fix link */}
                                Note: Events is the count of <Link to="#">all billable events</Link> which equate to
                                billable usage.
                            </Text>
                        }
                    />
                    <div className="d-flex justify-content-between text-muted mb-4">
                        <Text>
                            Showing {users.nodes.length} of {users.totalCount} users
                        </Text>
                        {users.nodes.length !== users.totalCount ? (
                            <Button variant="link" onClick={showMore}>
                                Show More
                            </Button>
                        ) : null}
                    </div>
                </>
            )}
        </div>
    )
}

function RenderUsernameAndEmail({ username, email, displayName, deletedAt }: SiteUser): JSX.Element {
    return (
        <Tooltip content={username}>
            <div className={classNames('d-flex flex-column p-2', styles.usernameColumn)}>
                {!deletedAt ? (
                    <Link to={`/users/${username}`} className="text-truncate">
                        @{username}
                    </Link>
                ) : (
                    <Text className="mb-0 text-truncate">@{username}</Text>
                )}
                <Text className="mb-0 text-truncate">{displayName}</Text>
                <Text className="mb-0 text-truncate">{email}</Text>
            </div>
        </Tooltip>
    )
}

type ActionHandler = (users: SiteUser[]) => void

interface UseUserListActionReturnType {
    handleForceSignOutUsers: ActionHandler
    handleDeleteUsers: ActionHandler
    handleDeleteUsersForever: ActionHandler
    handlePromoteToSiteAdmin: ActionHandler
    handleRevokeSiteAdmin: ActionHandler
    notification: { text: React.ReactNode; isError?: boolean } | undefined
    handleDismissNotification: () => void
    handleResetUserPassword: ActionHandler
}

const getUsernames = (users: SiteUser[]): string => users.map(user => user.username).join(', ')

function useUserListActions(onEnd: (error?: any) => void): UseUserListActionReturnType {
    const [forceSignOutUsers] = useMutation(FORCE_SIGN_OUT_USERS)
    const [deleteUsers] = useMutation(DELETE_USERS)
    const [deleteUsersForever] = useMutation(DELETE_USERS_FOREVER)

    const [notification, setNotification] = useState<UseUserListActionReturnType['notification']>()

    const handleDismissNotification = useCallback(() => setNotification(undefined), [])

    const onError = useCallback(
        (error: any) => {
            setNotification({
                text: (
                    <Text as="span">
                        Something went wrong :(!
                        <Text as="pre" className="m-1" size="small">
                            {error?.message}
                        </Text>
                    </Text>
                ),
                isError: true,
            })
            console.error(error)
            onEnd(error)
        },
        [onEnd]
    )

    const createOnSuccess = useCallback(
        (text: React.ReactNode, shouldReload = false) => () => {
            setNotification({ text })
            if (shouldReload) {
                onEnd()
            }
        },
        [onEnd]
    )

    const handleForceSignOutUsers = useCallback(
        (users: SiteUser[]) => {
            if (confirm('Are you sure you want to force sign out the selected user(s)?')) {
                forceSignOutUsers({ variables: { userIDs: users.map(user => user.id) } })
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully force signed out following {users.length} user(s):{' '}
                                <strong>{getUsernames(users)}</strong>
                            </Text>
                        )
                    )
                    .catch(onError)
            }
        },
        [forceSignOutUsers, onError, createOnSuccess]
    )

    const handleDeleteUsers = useCallback(
        (users: SiteUser[]) => {
            if (confirm('Are you sure you want to delete the selected user(s)?')) {
                deleteUsers({ variables: { userIDs: users.map(user => user.id) } })
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully deleted following {users.length} user(s):{' '}
                                <strong>{getUsernames(users)}</strong>
                            </Text>,
                            true
                        )
                    )
                    .catch(onError)
            }
        },
        [deleteUsers, onError, createOnSuccess]
    )
    const handleDeleteUsersForever = useCallback(
        (users: SiteUser[]) => {
            if (confirm('Are you sure you want to delete the selected user(s)?')) {
                deleteUsersForever({ variables: { userIDs: users.map(user => user.id) } })
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully deleted forever following {users.length} user(s):{' '}
                                <strong>{getUsernames(users)}</strong>
                            </Text>,
                            true
                        )
                    )
                    .catch(onError)
            }
        },
        [deleteUsersForever, onError, createOnSuccess]
    )

    const handlePromoteToSiteAdmin = useCallback(
        ([user]: SiteUser[]) => {
            if (confirm('Are you sure you want to promote the selected user to site admin?')) {
                setUserIsSiteAdmin(user.id, true)
                    .toPromise()
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully promoted user <strong>{user.username}</strong> to site admin.
                            </Text>,
                            true
                        )
                    )
                    .catch(onError)
            }
        },
        [onError, createOnSuccess]
    )

    const handleRevokeSiteAdmin = useCallback(
        ([user]: SiteUser[]) => {
            if (confirm('Are you sure you want to revoke the selected user from site admin?')) {
                setUserIsSiteAdmin(user.id, false)
                    .toPromise()
                    .then(
                        createOnSuccess(
                            <Text as="span">
                                Successfully revoked site admin from <strong>{user.username}</strong> user.
                            </Text>,
                            true
                        )
                    )
                    .catch(onError)
            }
        },
        [onError, createOnSuccess]
    )

    const handleResetUserPassword = useCallback(
        ([user]: SiteUser[]) => {
            if (confirm('Are you sure you want to reset the selected user password?')) {
                randomizeUserPassword(user.id)
                    .toPromise()
                    .then(({ resetPasswordURL }) => {
                        if (resetPasswordURL === null) {
                            createOnSuccess(
                                <Text as="span">
                                    Password was reset. The reset link was sent to the primary email of the user:{' '}
                                    <strong>{user.username}</strong>
                                </Text>
                            )()
                        } else {
                            createOnSuccess(
                                <>
                                    <Text>
                                        Password was reset. You must manually send <strong>{user.username}</strong> this
                                        reset link:
                                    </Text>
                                    <CopyableText text={resetPasswordURL} size={40} />
                                </>
                            )()
                        }
                    })
                    .catch(onError)
            }
        },
        [onError, createOnSuccess]
    )

    return {
        notification,
        handleForceSignOutUsers,
        handleDeleteUsers,
        handleDeleteUsersForever,
        handlePromoteToSiteAdmin,
        handleRevokeSiteAdmin,
        handleResetUserPassword,
        handleDismissNotification,
    }
}
