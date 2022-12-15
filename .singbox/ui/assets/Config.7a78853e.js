import{r as N,b as c,j as t,i as p,s as A,R as ee,c as te,k as ne,l as W,n as V,o as ae,h as B,d as U,q as oe,g as G,t as re,v as le,w as x,x as se,y as ie,z as ce,A as de,D as ue,u as pe,C as he,E as _,F as fe,B as y,G as ve,H as ge,J as me}from"./index.501e4fc6.js";import{r as be}from"./logs.73346e91.js";import{S as k}from"./Select.a8417f61.js";import{R as ye}from"./rotate-cw.efd67e7d.js";function ke(e,l){if(e==null)return{};var r=we(e,l),o,n;if(Object.getOwnPropertySymbols){var a=Object.getOwnPropertySymbols(e);for(n=0;n<a.length;n++)o=a[n],!(l.indexOf(o)>=0)&&(!Object.prototype.propertyIsEnumerable.call(e,o)||(r[o]=e[o]))}return r}function we(e,l){if(e==null)return{};var r={},o=Object.keys(e),n,a;for(a=0;a<o.length;a++)n=o[a],!(l.indexOf(n)>=0)&&(r[n]=e[n]);return r}var j=N.exports.forwardRef(function(e,l){var r=e.color,o=r===void 0?"currentColor":r,n=e.size,a=n===void 0?24:n,d=ke(e,["color","size"]);return c("svg",{ref:l,xmlns:"http://www.w3.org/2000/svg",width:a,height:a,viewBox:"0 0 24 24",fill:"none",stroke:o,strokeWidth:"2",strokeLinecap:"round",strokeLinejoin:"round",...d,children:[t("polyline",{points:"8 17 12 21 16 17"}),t("line",{x1:"12",y1:"12",x2:"12",y2:"21"}),t("path",{d:"M20.88 18.09A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.29"})]})});j.propTypes={color:p.exports.string,size:p.exports.oneOfType([p.exports.string,p.exports.number])};j.displayName="DownloadCloud";const Ce=j;function xe(e,l){if(e==null)return{};var r=_e(e,l),o,n;if(Object.getOwnPropertySymbols){var a=Object.getOwnPropertySymbols(e);for(n=0;n<a.length;n++)o=a[n],!(l.indexOf(o)>=0)&&(!Object.prototype.propertyIsEnumerable.call(e,o)||(r[o]=e[o]))}return r}function _e(e,l){if(e==null)return{};var r={},o=Object.keys(e),n,a;for(a=0;a<o.length;a++)n=o[a],!(l.indexOf(n)>=0)&&(r[n]=e[n]);return r}var I=N.exports.forwardRef(function(e,l){var r=e.color,o=r===void 0?"currentColor":r,n=e.size,a=n===void 0?24:n,d=xe(e,["color","size"]);return c("svg",{ref:l,xmlns:"http://www.w3.org/2000/svg",width:a,height:a,viewBox:"0 0 24 24",fill:"none",stroke:o,strokeWidth:"2",strokeLinecap:"round",strokeLinejoin:"round",...d,children:[t("path",{d:"M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"}),t("polyline",{points:"16 17 21 12 16 7"}),t("line",{x1:"21",y1:"12",x2:"9",y2:"12"})]})});I.propTypes={color:p.exports.string,size:p.exports.oneOfType([p.exports.string,p.exports.number])};I.displayName="LogOut";const Oe=I;function Se(e,l){if(e==null)return{};var r=Ne(e,l),o,n;if(Object.getOwnPropertySymbols){var a=Object.getOwnPropertySymbols(e);for(n=0;n<a.length;n++)o=a[n],!(l.indexOf(o)>=0)&&(!Object.prototype.propertyIsEnumerable.call(e,o)||(r[o]=e[o]))}return r}function Ne(e,l){if(e==null)return{};var r={},o=Object.keys(e),n,a;for(a=0;a<o.length;a++)n=o[a],!(l.indexOf(n)>=0)&&(r[n]=e[n]);return r}var P=N.exports.forwardRef(function(e,l){var r=e.color,o=r===void 0?"currentColor":r,n=e.size,a=n===void 0?24:n,d=Se(e,["color","size"]);return c("svg",{ref:l,xmlns:"http://www.w3.org/2000/svg",width:a,height:a,viewBox:"0 0 24 24",fill:"none",stroke:o,strokeWidth:"2",strokeLinecap:"round",strokeLinejoin:"round",...d,children:[t("polyline",{points:"3 6 5 6 21 6"}),t("path",{d:"M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"}),t("line",{x1:"10",y1:"11",x2:"10",y2:"17"}),t("line",{x1:"14",y1:"11",x2:"14",y2:"17"})]})});P.propTypes={color:p.exports.string,size:p.exports.oneOfType([p.exports.string,p.exports.number])};P.displayName="Trash2";const je=P,{useState:Ie,useRef:Pe,useEffect:Le,useCallback:ze}=ee;function O(e){return t("input",{className:A.input,...e})}function $e({value:e,...l}){const[r,o]=Ie(e),n=Pe(e);Le(()=>{n.current!==e&&o(e),n.current=e},[e]);const a=ze(d=>o(d.target.value),[o]);return t("input",{className:A.input,value:r,onChange:a,...l})}const Te="_root_9juo6_1",Re="_section_9juo6_2",De="_wrapSwitch_9juo6_27",Ee="_sep_9juo6_33",Fe="_label_9juo6_46",i={root:Te,section:Re,wrapSwitch:De,sep:Ee,label:Fe},Me="_fieldset_olb4q_1",We="_input_olb4q_10",Ve="_cnt_olb4q_10",S={fieldset:Me,input:We,cnt:Ve};function Ae({OptionComponent:e,optionPropsList:l,selectedIndex:r,onChange:o}){const n=te("visually-hidden",S.input),a=d=>{o(d.target.value)};return t("fieldset",{className:S.fieldset,children:l.map((d,m)=>c("label",{children:[t("input",{type:"radio",checked:r===m,name:"selection",value:m,"aria-labelledby":"traffic chart type "+m,onChange:a,className:n}),t("div",{className:S.cnt,children:t(e,{...d})})]},m))})}const{useMemo:Be}=B,Ue={plugins:{legend:{display:!1}},scales:{x:{display:!1,type:"category"},y:{display:!1,type:"linear"}}},H=[23e3,35e3,46e3,33e3,9e4,68e3,23e3,45e3],Ge=[184e3,183e3,196e3,182e3,19e4,186e3,182e3,189e3],He=H;function qe({id:e}){const l=ne.read(),r=Be(()=>({labels:He,datasets:[{...W,...V[e].up,data:H},{...W,...V[e].down,data:Ge}]}),[e]),o="chart-"+e;return ae(l.Chart,o,r,null,Ue),t("div",{style:{width:80,padding:5},children:t("canvas",{id:o})})}const{useEffect:q,useState:Je,useCallback:v,useRef:Qe}=B,Ke=[{id:0},{id:1},{id:2},{id:3}],Xe=[["debug","Debug"],["info","Info"],["warning","Warning"],["error","Error"],["silent","Silent"]],Ye=[{key:"port",label:"Http Port"},{key:"socks-port",label:"Socks5 Port"},{key:"mixed-port",label:"Mixed Port"},{key:"redir-port",label:"Redir Port"},{key:"mitm-port",label:"MITM Port"}],Ze=[["zh","\u4E2D\u6587"],["en","English"]],et=[["direct","Direct"],["rule","Rule"],["script","Script"],["global","Global"]],tt=[["gvisor","gVisor"],["system","System"],["lwip","LWIP"]],nt=e=>({configs:oe(e),apiConfig:G(e)}),at=e=>({selectedChartStyleIndex:ge(e),latencyTestUrl:me(e),apiConfig:G(e)}),ot=U(at)(lt),ut=U(nt)(rt);function rt({dispatch:e,configs:l,apiConfig:r}){return q(()=>{e(re(r))},[e,r]),t(ot,{configs:l})}function lt({dispatch:e,configs:l,selectedChartStyleIndex:r,latencyTestUrl:o,apiConfig:n}){var R,D,E,F;const[a,d]=Je(l),m=Qe(l);q(()=>{m.current!==l&&d(l),m.current=l},[l]);const J=v(()=>{e(le("apiConfig"))},[e]),w=v((s,u)=>{d({...a,[s]:u})},[a]),L=v((s,u)=>{const f={...a.tun,[s]:u};d({...a,tun:{...f}})},[a]),g=v(({name:s,value:u})=>{switch(s){case"mode":case"log-level":case"allow-lan":case"sniffing":w(s,u),e(x(n,{[s]:u})),s==="log-level"&&be({...n,logLevel:u});break;case"mitm-port":case"redir-port":case"socks-port":case"mixed-port":case"port":if(u!==""){const f=parseInt(u,10);if(f<0||f>65535)return}w(s,u);break;case"enable":case"stack":L(s,u),e(x(n,{tun:{[s]:u}}));break;default:return}},[n,e,w,L]),Q=v(s=>g(s.target),[g]),{selectChartStyleIndex:K,updateAppConfig:z}=se(),b=v(s=>{const u=s.target,{name:f,value:M}=u;switch(f){case"port":case"socks-port":case"mixed-port":case"redir-port":case"mitm-port":{const C=parseInt(M,10);if(C<0||C>65535)return;e(x(n,{[f]:C}));break}case"latencyTestUrl":{z(f,M);break}case"device name":case"interface name":break;default:throw new Error(`unknown input name ${f}`)}},[n,e,z]),X=v(()=>{e(ie(n))},[n,e]),Y=v(()=>{e(ce(n))},[n,e]),Z=v(()=>{e(de(n))},[n,e]),{data:$}=ue(["/version",n],()=>ve("/version",n)),{t:h,i18n:T}=pe();return c("div",{children:[t(he,{title:h("Config")}),c("div",{className:i.root,children:[Ye.map(s=>a[s.key]!==void 0?c("div",{children:[t("div",{className:i.label,children:s.label}),t(O,{name:s.key,value:a[s.key],onChange:Q,onBlur:b})]},s.key):null),c("div",{children:[t("div",{className:i.label,children:"Mode"}),t(k,{options:et,selected:a.mode.toLowerCase(),onChange:s=>g({name:"mode",value:s.target.value})})]}),c("div",{children:[t("div",{className:i.label,children:"Log Level"}),t(k,{options:Xe,selected:a["log-level"].toLowerCase(),onChange:s=>g({name:"log-level",value:s.target.value})})]}),c("div",{children:[t("div",{className:i.label,children:h("allow_lan")}),t("div",{className:i.wrapSwitch,children:t(_,{name:"allow-lan",checked:a["allow-lan"],onChange:s=>g({name:"allow-lan",value:s})})})]}),$.meta&&c("div",{children:[t("div",{className:i.label,children:h("tls_sniffing")}),t("div",{className:i.wrapSwitch,children:t(_,{name:"sniffing",checked:a.sniffing,onChange:s=>g({name:"sniffing",value:s})})})]})]}),t("div",{className:i.sep,children:t("div",{})}),$.meta&&c(fe,{children:[c("div",{className:i.section,children:[c("div",{children:[t("div",{className:i.label,children:h("enable_tun_device")}),t("div",{className:i.wrapSwitch,children:t(_,{checked:(R=a.tun)==null?void 0:R.enable,onChange:s=>g({name:"enable",value:s})})})]}),c("div",{children:[t("div",{className:i.label,children:"TUN IP Stack"}),t(k,{options:tt,selected:(E=(D=a.tun)==null?void 0:D.stack)==null?void 0:E.toLowerCase(),onChange:s=>g({name:"stack",value:s.target.value})})]}),c("div",{children:[t("div",{className:i.label,children:"Device Name"}),t(O,{name:"device name",value:(F=a.tun)==null?void 0:F.device,onChange:b})]}),c("div",{children:[t("div",{className:i.label,children:"Interface Name"}),t(O,{name:"interface name",value:a["interface-name"],onChange:b})]})]}),t("div",{className:i.sep,children:t("div",{})}),c("div",{className:i.section,children:[c("div",{children:[t("div",{className:i.label,children:"Reload"}),t(y,{start:t(ye,{size:16}),label:h("reload_config_file"),onClick:X})]}),c("div",{children:[t("div",{className:i.label,children:"GEO Databases"}),t(y,{start:t(Ce,{size:16}),label:h("update_geo_databases_file"),onClick:Y})]}),c("div",{children:[t("div",{className:i.label,children:"FakeIP"}),t(y,{start:t(je,{size:16}),label:h("flush_fake_ip_pool"),onClick:Z})]})]}),t("div",{className:i.sep,children:t("div",{})})]}),c("div",{className:i.section,children:[c("div",{children:[t("div",{className:i.label,children:h("latency_test_url")}),t($e,{name:"latencyTestUrl",type:"text",value:o,onBlur:b})]}),c("div",{children:[t("div",{className:i.label,children:h("lang")}),t("div",{children:t(k,{options:Ze,selected:T.language,onChange:s=>T.changeLanguage(s.target.value)})})]}),c("div",{children:[t("div",{className:i.label,children:h("chart_style")}),t(Ae,{OptionComponent:qe,optionPropsList:Ke,selectedIndex:r,onChange:K})]}),c("div",{children:[t("div",{className:i.label,children:"Action"}),t(y,{start:t(Oe,{size:16}),label:"Switch backend",onClick:J})]})]})]})}export{ut as default};
