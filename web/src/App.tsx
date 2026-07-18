import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { DocumentTitle } from '@/components/DocumentTitle'
import { AdminLayout } from '@/layouts/AdminLayout'
import { HomePage } from '@/pages/HomePage'
import { LoginPage } from '@/pages/LoginPage'
import { DashboardPage } from '@/pages/DashboardPage'
import { SpacesPage } from '@/pages/SpacesPage'
import { CardsPage } from '@/pages/CardsPage'
import { FilesPage } from '@/pages/FilesPage'
import { UploadsPage } from '@/pages/UploadsPage'

export default function App() {
  return (
    <BrowserRouter>
      <DocumentTitle />
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/admin/login" element={<LoginPage />} />
        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<DashboardPage />} />
          <Route path="spaces" element={<SpacesPage />} />
          <Route path="cards" element={<CardsPage />} />
          <Route path="files" element={<FilesPage />} />
          <Route path="uploads" element={<UploadsPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
