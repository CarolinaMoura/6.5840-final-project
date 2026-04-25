import React from 'react'
import ReactDOM from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import Home from './routes/Home'
import Room from './routes/Room'
import './index.css'

const router = createBrowserRouter([
  { path: '/', element: <Home /> },
  { path: '/rooms/:room', element: <Room /> },
])

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>,
)
