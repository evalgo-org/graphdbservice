    # pxgraphdb.default_remove("env-px-graphdb")
    # cnt_info = pxgraphdb.default("env-px-graphdb")
    # print(
    #     pxgraphdb.backup_restore(
    #         {'srv':environ.get('BKP_SRV'),'prefix':'dev', 'repos':['Consumption-Navigator-001'], 'user':environ.get('BKP_USER'), 'pass':environ.get('BKP_PASS')},
    #         {'srv':'http://172.18.0.4:7200'}
    #     )
    # )
    # print(
    #     pxgraphdb.export_import_repos(
    #         environ.get('BKP_SRV'), 
    #         'test', 
    #         ['Chatbot-Demo'], 
    #         'env-px-graphdb',
    #         pxgraphdb.PX_GRAPHDB_VOLUME, 
    #         pxgraphdb.PX_GRAPHDB_NETWORK, 
    #         environ.get('BKP_USER'), 
    #         environ.get('BKP_PASS')
    #     )
    # )
    # print(
    #     pxgraphdb.export_import_repos_graphs(
    #         environ.get('BKP_SRV'),
    #         'migration',
    #         'dataCatalog',
    #         ['https://data.kaeser.com/KKH/GPH'],
    #         'http://172.18.0.4:7200',
    #         'Chatbot-Demo',
    #         environ.get('BKP_USER'),
    #         environ.get('BKP_PASS')
    #     )
    # )
    # print(
    #     pxgraphdb.export_import_repos_graphs(
    #         'http://172.18.0.4:7200',
    #         'migration',
    #         'Chatbot-Demo',
    #         ['https://data.kaeser.com/KKH/GPH'],
    #         'http://172.18.0.4:7200',
    #         'Consumption-Navigator-001',
    #         environ.get('BKP_USER'),
    #         environ.get('BKP_PASS')
    #     )
    # )

    
    # cnt_info = pxgraphdb.default_ports(environ.get('RST_CONTAINER'), {7200: 7200})
    # pxgraphdb.backup_restore(environ.get('BKP_SRV'), 'local', repos, environ.get('BKP_USER'), environ.get('BKP_PASS'),'http://localhost:7200')
    
    # repos = ['Consumption-Navigator-001']
    # pxgraphdb.export_import_repos(environ.get('BKP_SRV'), 'local', repos, environ.get('RST_CONTAINER'), environ.get('RST_VOLUME'), environ.get('RST_NETWORK'), environ.get('BKP_USER'), environ.get('BKP_PASS'))

    # pxgraphdb.export_import_repos_c5_ke1(['Vestas-demo'])

    # pxgraphdb.export_import_repos_graphs(environ.get("BKP_SRV"), 'test-graphs', 'dataCatalog', ['https://vocab.kaeser.com/PIM/Units'], 'http://localhost:7200', 'Consumption-Navigator-001', environ.get("BKP_USER"), environ.get('BKP_PASS'))
