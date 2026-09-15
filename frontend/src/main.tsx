import React from 'react'
import * as ReactDOM from 'react-dom'
import * as antd from 'antd'
import 'antd/dist/reset.css'
import './style.css'

;(window as any).React = React
;(window as any).ReactDOM = ReactDOM
;(window as any).antd = antd

const runtime = document.createElement('script')
runtime.src = '/assets/runtime-app.js'
runtime.defer = true
document.body.appendChild(runtime)
