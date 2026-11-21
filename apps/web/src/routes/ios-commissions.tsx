import { createFileRoute } from '@tanstack/react-router'
import { NativeCommissions } from '../features/billing/commissions'
import css from '../features/billing/commissions.css?url'
export const Route=createFileRoute('/ios-commissions')({
  head:()=>({meta:[{title:'我的约稿'},{name:'robots',content:'noindex, nofollow'}],links:[{rel:'stylesheet',href:css}]}),
  component:NativeCommissions,
})
