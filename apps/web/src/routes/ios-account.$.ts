import { createFileRoute } from '@tanstack/react-router'
import { iosAccountRequest } from '../features/reader/ios-account.server'

export const Route = createFileRoute('/ios-account/$')({server:{handlers:{
  GET: ({params,request})=>iosAccountRequest(request,params._splat ?? ''),
  POST: ({params,request})=>iosAccountRequest(request,params._splat ?? ''),
  ANY: ()=>new Response(null,{status:405}),
}}})
