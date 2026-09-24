import { useState, useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { adminAPI } from '../api';
import { Project, Organization, ProjectUpdate } from '../types';

type TabKey = 'projects' | 'updates' | 'organizations';

const Admin = () => {
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState<TabKey>('projects');
  const [pendingProjects, setPendingProjects] = useState<Project[]>([]);
  const [pendingUpdates, setPendingUpdates] = useState<ProjectUpdate[]>([]);
  const [pendingOrgs, setPendingOrgs] = useState<Organization[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (user?.role === 'admin') {
      loadPendingItems();
    }
  }, [user]);

  const loadPendingItems = async () => {
    try {
      const [projectsRes, updatesRes, orgsRes] = await Promise.all([
        adminAPI.getPendingProjects(),
        adminAPI.getPendingUpdates(),
        adminAPI.getPendingOrganizations(),
      ]);
      setPendingProjects(projectsRes.data.projects);
      setPendingUpdates(updatesRes.data.updates);
      setPendingOrgs(orgsRes.data.organizations);
    } catch (error) {
      console.error('加载待审核内容失败:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleReviewProject = async (projectId: string, status: 'approved' | 'rejected') => {
    try {
      await adminAPI.reviewProject(projectId, { status, comment: '' });
      alert(`项目已${status === 'approved' ? '通过' : '拒绝'}`);
      loadPendingItems();
    } catch (error: any) {
      alert(error.response?.data?.message || '操作失败');
      loadPendingItems();
    }
  };

  const handleReviewOrg = async (orgId: string, status: 'approved' | 'rejected') => {
    try {
      await adminAPI.reviewOrganization(orgId, { status, comment: '' });
      alert(`组织已${status === 'approved' ? '通过' : '拒绝'}`);
      loadPendingItems();
    } catch (error: any) {
      alert(error.response?.data?.message || '操作失败');
      loadPendingItems();
    }
  };

  const handleApproveUpdate = async (update: ProjectUpdate) => {
    try {
      await adminAPI.reviewUpdate(update.id, { status: 'approved', comment: '' });
      alert('动态已通过');
      loadPendingItems();
    } catch (error: any) {
      if (error.response?.status === 409) {
        alert(`该动态已被其他管理员处理（当前结果：${reviewResultText(error.response?.data?.data?.update)}）`);
      } else {
        alert(error.response?.data?.message || '操作失败');
      }
      loadPendingItems();
    }
  };

  const handleRejectUpdate = async (update: ProjectUpdate) => {
    const reason = window.prompt('请填写驳回原因（将展示给提交组织）');
    if (reason === null) return; // 取消
    if (!reason.trim()) {
      alert('驳回必须填写原因');
      return;
    }
    try {
      await adminAPI.reviewUpdate(update.id, { status: 'rejected', comment: reason.trim() });
      alert('动态已驳回');
      loadPendingItems();
    } catch (error: any) {
      if (error.response?.status === 409) {
        alert(`该动态已被其他管理员处理（当前结果：${reviewResultText(error.response?.data?.data?.update)}）`);
      } else {
        alert(error.response?.data?.message || '操作失败');
      }
      loadPendingItems();
    }
  };

  if (!user || user.role !== 'admin') {
    return <Navigate to="/" />;
  }

  const categoryMap: Record<string, string> = {
    education: '助学',
    elderly: '助老',
    medical: '医疗',
    disaster: '救灾',
    environment: '环保',
    other: '其他',
  };

  return (
    <div>
      <h1 className="text-3xl font-bold text-gray-900 mb-8">管理后台</h1>

      <div className="flex gap-4 mb-8 flex-wrap">
        <button
          onClick={() => setActiveTab('projects')}
          className={`px-6 py-2 rounded-lg font-medium ${
            activeTab === 'projects'
              ? 'bg-primary-600 text-white'
              : 'bg-white text-gray-600 border border-gray-200'
          }`}
        >
          待审核项目 ({pendingProjects.length})
        </button>
        <button
          onClick={() => setActiveTab('updates')}
          className={`px-6 py-2 rounded-lg font-medium ${
            activeTab === 'updates'
              ? 'bg-primary-600 text-white'
              : 'bg-white text-gray-600 border border-gray-200'
          }`}
        >
          待审核动态 ({pendingUpdates.length})
        </button>
        <button
          onClick={() => setActiveTab('organizations')}
          className={`px-6 py-2 rounded-lg font-medium ${
            activeTab === 'organizations'
              ? 'bg-primary-600 text-white'
              : 'bg-white text-gray-600 border border-gray-200'
          }`}
        >
          待审核组织 ({pendingOrgs.length})
        </button>
      </div>

      {loading ? (
        <div className="text-center py-20">加载中...</div>
      ) : activeTab === 'projects' ? (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingProjects.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待审核项目</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingProjects.map((project) => (
                <div key={project.id} className="p-6">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2">
                        <span className="px-2 py-1 bg-blue-100 text-blue-700 rounded text-xs font-medium">
                          {categoryMap[project.category]}
                        </span>
                        <span className="text-sm text-gray-500">
                          发布时间：{new Date(project.createdAt).toLocaleDateString()}
                        </span>
                      </div>
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">{project.title}</h3>
                      <p className="text-gray-600 mb-2">{project.description}</p>
                      <div className="text-sm text-gray-500">
                        目标金额：¥{project.targetAmount.toLocaleString()}
                      </div>
                      <div className="text-sm text-gray-500">
                        执行计划：{project.executionPlan}
                      </div>
                    </div>
                    <div className="flex gap-2 ml-6">
                      <button
                        onClick={() => handleReviewProject(project.id, 'approved')}
                        className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100"
                      >
                        通过
                      </button>
                      <button
                        onClick={() => handleReviewProject(project.id, 'rejected')}
                        className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : activeTab === 'updates' ? (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingUpdates.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待审核动态</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingUpdates.map((update) => (
                <div key={update.id} className="p-6">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2">
                        <span className="px-2 py-1 bg-primary-100 text-primary-700 rounded text-xs font-medium">
                          项目动态
                        </span>
                        <span className="text-sm text-gray-500">
                          所属项目：{update.project?.title || update.projectId}
                        </span>
                        <span className="text-sm text-gray-500">
                          发起组织：{update.project?.organization?.name || '-'}
                        </span>
                        <span className="text-sm text-gray-500">
                          提交时间：{new Date(update.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">{update.title}</h3>
                      <p className="text-gray-600 whitespace-pre-wrap">{update.content}</p>
                    </div>
                    <div className="flex gap-2 ml-6">
                      <button
                        onClick={() => handleApproveUpdate(update)}
                        className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100"
                      >
                        通过
                      </button>
                      <button
                        onClick={() => handleRejectUpdate(update)}
                        className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
                      >
                        驳回（需填原因）
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingOrgs.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待审核组织</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingOrgs.map((org) => (
                <div key={org.id} className="p-6">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">{org.name}</h3>
                      <p className="text-gray-600 mb-2">{org.description}</p>
                      <div className="grid grid-cols-2 gap-4 text-sm text-gray-500">
                        <div>执照编号：{org.licenseNumber || '-'}</div>
                        <div>联系人：{org.contactPerson || '-'}</div>
                        <div>联系电话：{org.contactPhone || '-'}</div>
                        <div>地址：{org.address || '-'}</div>
                      </div>
                    </div>
                    <div className="flex gap-2 ml-6">
                      <button
                        onClick={() => handleReviewOrg(org.id, 'approved')}
                        className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100"
                      >
                        通过
                      </button>
                      <button
                        onClick={() => handleReviewOrg(org.id, 'rejected')}
                        className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

function reviewResultText(update?: ProjectUpdate): string {
  if (!update) return '已处理';
  if (update.status === 'approved') return '已通过';
  if (update.status === 'rejected') return `已驳回${update.reviewComment ? `，原因：${update.reviewComment}` : ''}`;
  return update.status;
}

export default Admin;
